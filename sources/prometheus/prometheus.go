/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package prometheus

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	//"net/http"
	"net/url"
	//"os"
	"text/template"
	"time"

	//"github.com/prometheus/common/config"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	appmodel "opsani-ignite/app/model"
	"opsani-ignite/log"
	opsmath "opsani-ignite/math"
)

// Example API usage: https://github.com/prometheus/client_golang/blob/master/api/prometheus/v1/example_test.go
// Note:
//    Query() returns model.Vector
//    QueryRange() returns model.Matrix
//    In turn, these have pointers to SampleStream
//    In turn, the stream contains (a) Metric (name and LabelSet) and (b) Values of type []SamplePair
//    In turn, SamplePair contains (a) TimeStamp and (b) SampleValue (typically a float)
// Use the following command to get relevant references:
//    go doc v1.API.{Query|QueryValues|LabelValues}
//    go doc model.{Matrix|Vector|SampleStream|SamplePair|Metric|Value|LabelSet|LabelValue}

const queryTimeout = 10*time.Second		// TODO: consider making configurable

type QuerySelectors struct {
	appmodel.AppMetadata
	PodSelector string
}

func createAPI(uri *url.URL) (v1.API, error) {
	client, err := api.NewClient(api.Config{
		Address: uri.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating Prometheus client: %v\n", err)
	}

	return v1.NewAPI(client), nil
}

func collectNamespaces(ctx context.Context, promApi v1.API, timeRange v1.Range) (model.LabelValues, v1.Warnings, error) {
	// set up query context with timeout
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// Collect namespaces
	rawNamespaces, warnings, err := promApi.LabelValues(ctx, "namespace", []string{}, timeRange.Start, timeRange.End)
	if err != nil {
		return nil, nil, fmt.Errorf("Error querying Prometheus: %v\n", err)
	}
	if len(warnings) > 0 {
		log.Warnf("Warnings: %v\n", warnings)
	}
	namespaces := make([]model.LabelValue, 0, len(rawNamespaces))
	for _, n := range rawNamespaces {
		switch n {
		case "kube-system", "kube-public", "kube-node-lease":
			continue
		}
		namespaces = append(namespaces, n)
	}

	return namespaces, warnings, nil
}

func getAggregateMetric(ctx context.Context, promApi v1.API, app *appmodel.App, timeRange v1.Range, metric string, aggrFunc string) (*float64, v1.Warnings, error) {
	// set up query context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	// prepare query string
	podRE := fmt.Sprintf("%v-.*", app.Metadata.Workload) // pod naming template is <deployment_name>-<pod_spec_hash>-<pod_unique_id> - TODO: tighten RE to avoid unlikely conflicts
	query := fmt.Sprintf("%v(%v{namespace=%q,pod=~%q})", aggrFunc, metric, app.Metadata.Namespace, podRE)

	// Collect values
	result, warnings, err := promApi.QueryRange(ctx, query, timeRange)
	if err != nil {
		return nil, nil, fmt.Errorf("Error querying Prometheus for %q: %v\n", query, err)
	}
	if len(warnings) > 0 {
		log.Warnf("Warnings: %v\n", warnings)
	}

	//log.Tracef("Application %v:%v: query %v(%v):\n\t%T : %v\n\n", app.Metadata.Namespace, app.Metadata.Workload, aggrFunc, metric, result, result)

	// Parse results as a list of series
	series, ok := result.(model.Matrix)
	if !ok {
		return nil, warnings, fmt.Errorf("Query %q returned %T instead of Matrix; assuming no data", query, result)
	}
	if len(series) == 0 {
		return nil, warnings, nil
	}
	if len(series) != 1 {
		return nil, warnings, fmt.Errorf("Query %q returned %v instead of a single series (%v); treating as if no data", query, len(series), series)
	}

	// evaluate the response by series label names -- here it should be empty
	if len(series[0].Metric) != 0 {
		return nil, warnings, fmt.Errorf("Query %q returned non-empty labels (%v) for the single series; treating as if no data", query, series[0].Metric)
	}

	// Aggregate across returned values
	values := []float64{}
	for _, v := range series[0].Values {
		values = append(values, float64(v.Value)) // ignoring Timestamps
	}
	var value float64
	switch aggrFunc {
	case "min":
		value = opsmath.Min(values...)
	case "sum":
		value = opsmath.Sum(values...)
	default:
		return nil, warnings, fmt.Errorf("Query %q uses not-yet-supported aggregation function %q", query, aggrFunc)
	}

	return &value, warnings, nil
}

func getRangedMetric(ctx context.Context, promApi v1.API, app *appmodel.App, timeRange v1.Range, queryTemplate *template.Template, querySelectors *QuerySelectors) (*float64, v1.Warnings, error) {
	// set up query context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

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
		return nil, nil, fmt.Errorf("Error querying Prometheus for %q: %v\n", query, err)
	}
	if len(warnings) > 0 {
		log.Warnf("Warnings: %v\n", warnings)
	}

	log.Tracef("Application %v:%v: query %q:\n\t%T : %v\n\n", app.Metadata.Namespace, app.Metadata.Workload, query, result, result)

	// Parse results as a list of series
	series, ok := result.(model.Matrix)
	if !ok {
		return nil, warnings, fmt.Errorf("Query %q returned %T instead of Matrix; assuming no data", query, result)
	}
	if len(series) == 0 {
		return nil, warnings, nil
	}
	if len(series) != 1 {
		return nil, warnings, fmt.Errorf("Query %q returned %v instead of a single series (%v); treating as if no data", query, len(series), series)
	}

	// evaluate the response by series label names -- here it should be empty
	// TODO: some queries result in no labels (e.g., cpu utilization) but others have them (e.g., replica count)
	//if len(series[0].Metric) != 0 {
	//	return nil, warnings, fmt.Errorf("Query %q returned non-empty labels (%v) for the single series; treating as if no data", query, series[0].Metric)
	//}

	// Aggregate across returned values
	values := []float64{}
	for _, v := range series[0].Values {
		values = append(values, float64(v.Value)) // ignoring Timestamps
	}
	value := opsmath.Avg(values...) // prepared for other aggregations

	return &value, warnings, nil
}

func collectDeploymentDetails(ctx context.Context, promApi v1.API, app *appmodel.App, timeRange v1.Range) (v1.Warnings, error) {
	allWarnings := v1.Warnings{}

	// prepare query selectors
	// Note: pod naming template is <deployment_name>-<pod_spec_hash>-<pod_unique_id>
	//       TODO: tighten RE to avoid unlikelyconflicts
	// TODO: deal with `container` label for aggregated pod metrics
	podRegexp := fmt.Sprintf("%v-.*", app.Metadata.Workload)
	podSelector := fmt.Sprintf("namespace=%q,pod=~%q", app.Metadata.Namespace, podRegexp)
	selectors := QuerySelectors{
		app.Metadata,
		podSelector,
	}

	// determine presence of writeable volumes
	res, warnings, err := getAggregateMetric(ctx, promApi, app, timeRange, "kube_pod_spec_volumes_persistentvolumeclaims_readonly", "min")
	if err != nil {
		log.Errorf("Error querying Prometheus for volume access %v: %v\n", app.Metadata, err)
	} else {
		if len(warnings) > 0 {
			allWarnings = append(allWarnings, warnings...)
			log.Warnf("Warnings during volume info collection: %v\n", warnings)
		}
		if res != nil && *res == 0 {
			app.Settings.WriteableVolume = true
		}
	}

	// collect replicas
	replicas, warnings, err := getRangedMetric(ctx, promApi, app, timeRange, replicaCountTemplate, &selectors)
	if err != nil {
		log.Errorf("Error querying Prometheus for replica count %v: %v\n", app.Metadata, err)
	} else {
		if len(warnings) > 0 {
			allWarnings = append(allWarnings, warnings...)
			log.Warnf("Warnings during replica counts collection: %v\n", warnings)
		}
		if replicas != nil {
			app.Metrics.AverageReplicas = *replicas
		}
	}

	// collect container info
	warnings, err = collectContainersInfo(ctx, promApi, app, timeRange)
	if err != nil {
		log.Errorf("Error querying Prometheus for workload's containers info %v: %v\n", app.Metadata, err)
	} else {
		if len(warnings) > 0 {
			allWarnings = append(allWarnings, warnings...)
			log.Warnf("Warnings during workload's container info collection: %v\n", warnings)
		}
	}

	// collect usage
	cpuUsed, warnings, err := getRangedMetric(ctx, promApi, app, timeRange, cpuUtilizationTemplate, &selectors)
	if err != nil {
		log.Errorf("Error querying Prometheus for CPU utilization %v: %v\n", app.Metadata, err)
	} else {
		if len(warnings) > 0 {
			allWarnings = append(allWarnings, warnings...)
			log.Warnf("Warnings during cpu utilization collection: %v\n", warnings)
		}
		if cpuUsed != nil {
			app.Metrics.CpuUtilization = *cpuUsed
		}
	}
	memoryUsed, warnings, err := getRangedMetric(ctx, promApi, app, timeRange, memoryUtilizationTemplate, &selectors)
	if err != nil {
		log.Errorf("Error querying Prometheus for memory utilization %v: %v\n", app.Metadata, err)
	} else {
		if len(warnings) > 0 {
			allWarnings = append(allWarnings, warnings...)
			log.Warnf("Warnings during memory utilization collection: %v\n", warnings)
		}
		if memoryUsed != nil {
			app.Metrics.MemoryUtilization = *memoryUsed
		}
	}

	return allWarnings, nil
}

func mapNamespace(ctx context.Context, promApi v1.API, namespace model.LabelValue, timeRange v1.Range, progressCallback log.ProgressUpdateFunc) (apps []*appmodel.App) {
	apps = []*appmodel.App{}

	// set up query context with timeout
	reqCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// Collect values
	// TODO: consider santizing namespace value despite using %q and model.LabelValue
	result, warnings, err := promApi.Query(reqCtx, fmt.Sprintf("kube_deployment_labels{namespace=%q}", namespace), timeRange.End)
	if err != nil {
		log.Errorf("Error querying Prometheus for namespace %q: %v\n", namespace, err)
		return
	}
	if len(warnings) > 0 {
		log.Warnf("Warnings: %v\n", warnings)
	}

	// Collect deployments
	samples, ok := result.(model.Vector)
	if !ok {
		log.Errorf("Unexpected deployment query result: got type %T, expected Vector", result)
		return
	}
	for _, w := range samples {
		workload := w.Metric["deployment"]
		apps = append(apps, &appmodel.App{Metadata: appmodel.AppMetadata{Namespace: string(namespace), Workload: string(workload), WorkloadKind: "Deployment", WorkloadApiVersion: "apps/v1"}})
		//log.Tracef("namespace %q: deployment %q\n", namespace, workload)
	}

	// indicate progress: total newly discovered apps
	if progressCallback != nil {
		progressCallback(log.ProgressInfo{WorkloadsTotal: len(apps)}, true)
	}

	// Fill in deployment details
	for _, app := range apps {
		warnings, err := collectDeploymentDetails(ctx, promApi, app, timeRange)
		if len(warnings) > 0 {
			log.Warnf("Warnings: %v\n", warnings)
		}
		if err != nil {
			log.Errorf("Failed to collect deployment details for app %v: %v\n", app.Metadata, err)
		}

		//log.Tracef("%#v\n\n", app)
		if progressCallback != nil {
			progressCallback(log.ProgressInfo{WorkloadsDone: 1}, true)
		}
	}

	// indicate progress: namespace completed
	if progressCallback != nil {
		progressCallback(log.ProgressInfo{NamespacesDone: 1}, true)
	}

	return apps
}

func collectSingleApp(ctx context.Context, promApi v1.API, namespace string, timeRange v1.Range, workload string, workloadApiVersion string, workloadKind string) *appmodel.App {
	app := &appmodel.App{
		Metadata: appmodel.AppMetadata{
			Namespace:          string(namespace),
			Workload:           workload,
			WorkloadKind:       workloadKind,
			WorkloadApiVersion: workloadApiVersion,
		}}

	// TODO: check if the application exists, return if not
	if false {
		return nil
	}

	// Fill in deployment details
	warnings, err := collectDeploymentDetails(ctx, promApi, app, timeRange)
	if len(warnings) > 0 {
		log.Warnf("Warnings: %v\n", warnings)
	}
	if err != nil {
		log.Errorf("Failed to collect deployment details for app %v: %v\n", app.Metadata, err)
	}

	//log.Tracef("%#v\n\n", app)

	return app
}

// In parallel, collect the workloads in each namespace
func collectMultipleApps(
	ctx context.Context, 
	promApi v1.API, 
	namespaces []model.LabelValue, 
	timeRange v1.Range,
	progressCallback log.ProgressUpdateFunc,
) []*appmodel.App {
	// map applications in each namespace, in a goroutine per namespace
	lists := make(chan []*appmodel.App)
	var wg sync.WaitGroup
	wg.Add(len(namespaces))
	for _, n := range namespaces {
		go func(namespace model.LabelValue) {
			defer wg.Done()
			lists <- mapNamespace(ctx, promApi, namespace, timeRange, progressCallback)
		}(n)
	}

	// collect apps into a single list, waiting for all namespaces to finish
	finalList := make(chan []*appmodel.App)
	go func() {
		apps := make([]*appmodel.App, 0)
		for list := range lists {
			for _, app := range list {
				//log.Tracef("Reduced app %v:%v\n", app.Metadata.Namespace, app.Metadata.Workload)
				apps = append(apps, app)
			}
		}
		finalList <- apps
	}()
	wg.Wait()
	close(lists)

	// get list from parallel rendezvous (single element in the channel, containing the whole list)
	apps := []*appmodel.App{}
	for _, app := range <-finalList {
		apps = append(apps, app)
		//log.Tracef("Found app %v:%v\n", app.Metadata.Namespace, app.Metadata.Workload)
	}
	close(finalList)

	return apps
}

func Init() {
	initializeTemplates()
}

func PromGetAll(
	ctx context.Context,
	promUri *url.URL,
	namespace string,
	workload string,
	workloadApiVersion string,
	workloadKind string,
	timeStart time.Time,
	timeEnd time.Time,
	timeStep time.Duration,
	progressCallback log.ProgressUpdateFunc,
) ([]*appmodel.App, error) {
	// set up API client
	promApi, err := createAPI(promUri)
	if err != nil {
		return nil, err
	}

	// choose a time range
	timeRange := v1.Range{
		Start: timeStart,
		End:   timeEnd,
		Step:  timeStep,
	}

	// Collect namespaces
	var namespaces []model.LabelValue
	if namespace == "" {
		var warnings v1.Warnings
		var err error
		namespaces, warnings, err = collectNamespaces(ctx, promApi, timeRange)
		if err != nil {
			return nil, fmt.Errorf("Error querying Prometheus: %v\n", err)
		}
		if len(warnings) > 0 {
			log.Warnf("Warnings: %v\n", warnings)
		}
	} else {
		namespaces = []model.LabelValue{model.LabelValue(namespace)}
	}
	log.Tracef("Namespaces: %v", namespaces)
	if progressCallback != nil {
		progressCallback(log.ProgressInfo{NamespacesTotal: len(namespaces)}, false)
	}


	var apps []*appmodel.App
	if workload == "" {
		apps = collectMultipleApps(ctx, promApi, namespaces, timeRange, progressCallback)
	} else {
		if progressCallback != nil {
			progressCallback(log.ProgressInfo{WorkloadsTotal: 1}, true)
		}
		apps = []*appmodel.App{
			collectSingleApp(ctx, promApi, namespace, timeRange, workload, workloadKind, workloadApiVersion),
		}
		if progressCallback != nil {
			progressCallback(log.ProgressInfo{NamespacesDone: 1, WorkloadsDone: 1}, true)
			}
	}

	return apps, nil
}
