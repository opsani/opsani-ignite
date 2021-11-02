/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package prometheus

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"text/template"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	appmodel "opsani-ignite/app/model"
	"opsani-ignite/log"
	opsmath "opsani-ignite/math"
)

// get request or limit values for all resources of all containers of the specificed application
func getContainersResources(ctx context.Context, promApi v1.API, app *appmodel.App, timeRange v1.Range, queryTemplate *template.Template, querySelectors *QuerySelectors, resourceFieldName string) (v1.Warnings, error) {
	// set up query context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // TODO: fix constant
	defer cancel()

	// prepare query string by injecting selector data into the provided query template
	var buf bytes.Buffer
	err := queryTemplate.Execute(&buf, querySelectors)
	if err != nil {
		return nil, fmt.Errorf("Error preparing query: %v\n", err)
	}
	query := buf.String()

	// Collect values
	result, warnings, err := promApi.Query(ctx, query, timeRange.End)
	if err != nil {
		return nil, fmt.Errorf("Error querying Prometheus for %q: %v\n", query, err)
	}
	if len(warnings) > 0 {
		log.Warnf("Warnings: %v\n", warnings)
	}

	log.Tracef("Application %v:%v: query %q:\n\t%T : %v\n\n", app.Metadata.Namespace, app.Metadata.Workload, query, result, result)

	// Parse results as a map of values
	list, ok := result.(model.Vector)
	if !ok {
		return warnings, fmt.Errorf("Query %q returned %T instead of Vector; assuming no data", query, result)
	}
	if len(list) == 0 {
		//TODO: add warning that we didn't find this info (which is OK - it may not be set)
		return warnings, nil
	}

	// evaluate the response as a list of <container, resource> tuples (labels) and a single value for each
	for _, c := range list {
		// evaluate response by series labels (must match query)
		labels := c.Metric
		nameLabel, ok1 := labels["container"]
		resourceLabel, ok2 := labels["resource"]
		if len(labels) != 2 || !ok1 || !ok2 {
			return warnings, fmt.Errorf("Query %q returned labels %v, expected %v", query, labels, []string{"container", "resource"})
		}
		name, resource := string(nameLabel), string(resourceLabel)
		if resource != "cpu" && resource != "memory" {
			log.Warnf("Query %q returned unrecognized resource type %q, ignoring", query, resource)
			continue
		}
		value := float64(c.Value)

		// update container info
		found := false
		for index, info := range app.Containers {
			if name != info.Name {
				continue
			}
			found = true
			log.Tracef("App %v, %v.%v.%v = %v", app.Metadata, name, resource, resourceFieldName, value)

			// update container info sub-structure (Cpu or Memory), relying on resource name match and setting request/limit
			// essentially, info.{Cpu|Memory}.{Limit|Request} = value
			infoValue := reflect.ValueOf(&info).Elem()
			resourceStruct := infoValue.FieldByName(strings.Title(resource))
			resourceValue := resourceStruct.FieldByName(resourceFieldName)
			resourceValue.Set(reflect.ValueOf(value))
			app.Containers[index] = info
			break
		}
		if !found {
			// TODO: add warnings
			log.Warnf("Unexpected combination of container/resource: %q/%q; ignoring value", name, resource)
		}
	}

	return nil, nil
}

func getContainersUseValueMap(ctx context.Context, promApi v1.API, app *appmodel.App, timeRange v1.Range, queryTemplate *template.Template, querySelectors *QuerySelectors) (map[string]float64, v1.Warnings, error) {
	// set up query context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // TODO: fix constant
	defer cancel()

	var allWarnings v1.Warnings

	// prepare query string by injecting selector data into the provided query template
	var buf bytes.Buffer
	err := queryTemplate.Execute(&buf, querySelectors)
	if err != nil {
		return nil, nil, fmt.Errorf("Error preparing query: %v\n", err)
	}
	query := buf.String()

	// Collect values
	result, warnings, err := promApi.QueryRange(ctx, query, timeRange)
	if err != nil {
		return nil, warnings, fmt.Errorf("Error querying Prometheus for %q: %v\n", query, err)
	}
	if len(warnings) > 0 {
		log.Warnf("Warnings: %v\n", warnings)
		allWarnings = append(allWarnings, warnings...)
	}

	// Parse results as a list of series
	series, ok := result.(model.Matrix)
	if !ok {
		return nil, warnings, fmt.Errorf("Query %q returned %T instead of Matrix; assuming no data", query, result)
	}
	if len(series) == 0 {
		return nil, warnings, nil
	}

	// aggregate and distribute values by container name
	valueMap := make(map[string]float64, len(app.Containers))
	for _, c := range series { // c is *model.SampleStream
		if len(c.Metric) > 1 {
			log.Warnf("metrics returned for query %q contain labels %v, expected %v, ignoring extras (app %v)", query, c.Metric, []string{"container"}, app.Metadata)
		}
		if len(c.Metric) == 0 {
			//log.Tracef("(expected) skipping metrics for without labels for app %v, query %q", app.Metadata, query)
			continue
		}
		// extract container name
		name, ok := c.Metric["container"]
		if !ok {
			log.Errorf("metric returned for query %q does not contain the %q label for app %v, required; skipping series ", query, "container", app.Metadata)
			continue
		}
		if name == "" || name == "POD" {
			//log.Tracef("(expected) skipping metrics for container label value %q for app %v, query %q", name, app.Metadata, query)
			continue
		}

		// process statistics over the values
		value, warnigns, err := valueFromSamplePairs(c.Values, fmt.Sprintf("app %v, container %q, query %q", app.Metadata, name, query))
		if err != nil {
			// convert to warning
			msg := fmt.Sprintf("Failed statistical processing for app %v, container %q, query %q results: %v; skipping series", app.Metadata, name, query, err)
			warnings = append(warnigns, msg)
			allWarnings = append(allWarnings, warnigns...)
			log.Errorf("%v", msg)
			continue
		}
		if len(warnigns) > 0 {
			allWarnings = append(allWarnings, warnings...)
		}
		valueMap[string(name)] = value
	}

	return valueMap, warnings, nil
}

func getContainersUse(ctx context.Context, promApi v1.API, app *appmodel.App, timeRange v1.Range, queryTemplate *template.Template, querySelectors *QuerySelectors, resource string, field string) (v1.Warnings, error) {
	// get container usage metric into valuemap by container name
	valueMap, warnings, err := getContainersUseValueMap(ctx, promApi, app, timeRange, queryTemplate, querySelectors)
	if err != nil {
		return warnings, err
	}

	// distribute values
	for i := range app.Containers {
		contName := app.Containers[i].Name
		v, ok := valueMap[contName]
		if !ok {
			if field != "SecondsThrottled" { // seconds throttled may be undefined if no throttling has occurred
				log.Warnf("Didn't find value of %v.%v for container %q of app %v; assuming 0", resource, field, contName, app.Metadata)
			}
		} else {
			// app.Containers[i].<resource>.<field> = v
			containerStruct := reflect.ValueOf(&app.Containers[i]).Elem()
			var resourceValue reflect.Value
			if resource != "" {
				resourceStruct := containerStruct.FieldByName(strings.Title(resource))
				resourceValue = resourceStruct.FieldByName(field)
			} else {
				resourceValue = containerStruct.FieldByName(field)
			}
			resourceValue.Set(reflect.ValueOf(v))

			delete(valueMap, contName)
		}
	}
	if len(valueMap) > 0 {
		msg := fmt.Sprintf("Unexpected container data series for %v.%v (app %v): %v; ignoring", resource, field, app.Metadata, valueMap)
		warnings = append(warnings, msg)
		log.Warnf(msg)
	}

	return warnings, nil
}

func handleWarnErr(allWarnings v1.Warnings, newWarnings v1.Warnings, newError error, app *appmodel.App, label string) v1.Warnings {
	if newError != nil {
		msg := fmt.Sprintf("Error querying Prometheus for %v on app %v: %v; skipping value", label, app.Metadata, newError)
		allWarnings = append(allWarnings, msg)
		log.Error(msg)
	} else if len(newWarnings) > 0 {
		allWarnings = append(allWarnings, newWarnings...)
		log.Warnf("Warnings while querying Prometheus for %v on app %v: %v", label, app.Metadata, newWarnings)
	}
	return allWarnings
}

func collectContainersInfo(ctx context.Context, promApi v1.API, app *appmodel.App, timeRange v1.Range) (v1.Warnings, error) {
	// set up query context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // TODO: fix constant
	defer cancel()

	var allWarnings v1.Warnings

	// --- Get container info

	// prepare query selectors (TODO: refactor to a helper)
	// Note: pod naming template is <deployment_name>-<pod_spec_hash>-<pod_unique_id>
	//       TODO: tighten RE to avoid unlikelyconflicts
	podRegexp := fmt.Sprintf("%v-.*", app.Metadata.Workload)
	podSelector := fmt.Sprintf("namespace=%q,pod=~%q", app.Metadata.Namespace, podRegexp)
	selectors := QuerySelectors{
		app.Metadata,
		podSelector,
	}

	// prepare query string by injecting selector data into the provided query template
	var buf bytes.Buffer
	err := containerInfoTemplate.Execute(&buf, selectors)
	if err != nil {
		return nil, fmt.Errorf("Error preparing query: %v\n", err)
	}
	query := buf.String()

	// Collect values
	result, warnings, err := promApi.Query(ctx, query, timeRange.End)
	if err != nil {
		return nil, fmt.Errorf("Error querying Prometheus for %q: %v\n", query, err)
	}
	if len(warnings) > 0 {
		log.Warnf("Warnings: %v\n", warnings)
	}

	log.Tracef("Application %v:%v: query %q:\n\t%T : %v\n\n", app.Metadata.Namespace, app.Metadata.Workload, query, result, result)

	// Parse results as a list of series
	series, ok := result.(model.Vector)
	if !ok {
		return warnings, fmt.Errorf("Query %q returned %T instead of Vector; assuming no data", query, result)
	}
	if len(series) == 0 {
		return warnings, nil
	}
	for _, c := range series {
		labels := c.Metric
		name, ok := labels["container"]
		if len(labels) != 1 || !ok {
			return warnings, fmt.Errorf("Query %q returned labels %v, expected %v", query, labels, []string{"container"})
		}
		container := appmodel.AppContainer{Name: string(name)}
		container.Cpu.Unit = "cores"
		container.Memory.Unit = "bytes"
		app.Containers = append(app.Containers, container)
	}

	// Get restart counts
	warnings, err = getContainersUse(ctx, promApi, app, timeRange, containerRestartsTemplate, &selectors, "", "RestartCount")
	allWarnings = handleWarnErr(allWarnings, warnings, err, app, "restart counts")

	// --- Get resource specifications

	// Get resource requests
	warnings, err = getContainersResources(ctx, promApi, app, timeRange, containerResourceRequestsTemplate, &selectors, "Request")
	allWarnings = handleWarnErr(allWarnings, warnings, err, app, "resource requests")

	// Get resource limits
	warnings, err = getContainersResources(ctx, promApi, app, timeRange, containerResourceLimitsTemplate, &selectors, "Limit")
	allWarnings = handleWarnErr(allWarnings, warnings, err, app, "resource limits")

	// --- Get usage metrics

	// Get resource usage
	warnings, err = getContainersUse(ctx, promApi, app, timeRange, containerCpuUseTemplate, &selectors, "Cpu", "Usage")
	allWarnings = handleWarnErr(allWarnings, warnings, err, app, "CPU usage")
	warnings, err = getContainersUse(ctx, promApi, app, timeRange, containerMemoryUseTemplate, &selectors, "Memory", "Usage")
	allWarnings = handleWarnErr(allWarnings, warnings, err, app, "memory usage")

	// Get resource saturation
	warnings, err = getContainersUse(ctx, promApi, app, timeRange, containerCpuSaturationTemplate, &selectors, "Cpu", "Saturation")
	allWarnings = handleWarnErr(allWarnings, warnings, err, app, "CPU saturation")
	warnings, err = getContainersUse(ctx, promApi, app, timeRange, containerMemorySaturationTemplate, &selectors, "Memory", "Saturation")
	allWarnings = handleWarnErr(allWarnings, warnings, err, app, "memory saturation")

	// Get CPU throttling stats
	warnings, err = getContainersUse(ctx, promApi, app, timeRange, containerCpuSecondsThrottledTemplate, &selectors, "Cpu", "SecondsThrottled")
	allWarnings = handleWarnErr(allWarnings, warnings, err, app, "CPU throttling")

	// Get network traffic stats (pod-level, not container-level)
	rxRate, warnings, err := getRangedMetric(ctx, promApi, app, timeRange, containerRxPacketsTemplate, &selectors)
	allWarnings = handleWarnErr(allWarnings, warnings, err, app, "Received packets rate")
	txRate, warnings, err := getRangedMetric(ctx, promApi, app, timeRange, containerTxPacketsTemplate, &selectors)
	allWarnings = handleWarnErr(allWarnings, warnings, err, app, "Transmitted packets rate")
	if rxRate != nil {
		app.Metrics.PacketReceiveRate = opsmath.MagicRound(*rxRate)
	}
	if txRate != nil {
		app.Metrics.PacketTransmitRate = opsmath.MagicRound(*txRate)
	}

	log.Tracef("App %v has %v container(s): %v", app.Metadata, len(app.Containers), app.Containers)

	return nil, nil
}
