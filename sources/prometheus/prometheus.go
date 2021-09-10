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
)

// Example API usage: https://github.com/prometheus/client_golang/blob/master/api/prometheus/v1/example_test.go
// See also: https://stackoverflow.com/questions/63471775/extract-prometheus-metrics-in-go

type QuerySelectors struct {
	appmodel.AppMetadata
	PodSelector string
}

func min(samples ...float64) float64 {
	min := samples[0]
	for _, val := range samples[1:] {
		if val < min {
			min = val
		}
	}
	return min
}

func sum(samples ...float64) float64 {
	total := 0.0
	for _, val := range samples {
		total += val
	}
	return total
}

func avg(samples ...float64) float64 {
	if len(samples) == 0 {
		return 0.0
	}
	total := 0.0
	for _, val := range samples {
		total += val
	}
	return total / float64(len(samples))
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

func collectNamespaces(promApi v1.API, timeRange v1.Range) (model.LabelValues, v1.Warnings, error) {
	// set up query context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // TODO: fix constant
	defer cancel()

	// Collect namespaces
	rawNamespaces, warnings, err := promApi.LabelValues(ctx, "namespace", []string{}, timeRange.Start, timeRange.End)
	if err != nil {
		return nil, nil, fmt.Errorf("Error querying Prometheus: %v\n", err)
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
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

func getAggregateMetric(promApi v1.API, ctx context.Context, app *appmodel.App, timeRange v1.Range, metric string, aggrFunc string) (*float64, v1.Warnings, error) {
	// set up query context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // TODO: fix constant
	defer cancel()

	// prepare query string
	pod_re := fmt.Sprintf("%v-.*", app.Metadata.Workload) // pod naming template is <deployment_name>-<pod_spec_hash>-<pod_unique_id> - TODO: tighten RE to avoid unlikelyconflicts
	query := fmt.Sprintf("%v(%v{namespace=%q,pod=~%q})", aggrFunc, metric, app.Metadata.Namespace, pod_re)

	// Collect values
	result, warnings, err := promApi.QueryRange(ctx, query, timeRange)
	if err != nil {
		return nil, nil, fmt.Errorf("Error querying Prometheus for %q: %v\n", query, err)
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}

	//fmt.Printf("Application %v:%v: query %v(%v):\n\t%T : %v\n\n", app.Metadata.Namespace, app.Metadata.Workload, aggrFunc, metric, result, result)

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
		value = min(values...)
	case "sum":
		value = sum(values...)
	default:
		return nil, warnings, fmt.Errorf("Query %q uses not-yet-supported aggregation function %q", query, aggrFunc)
	}

	return &value, warnings, nil
}

func getRangedMetric(promApi v1.API, ctx context.Context, app *appmodel.App, timeRange v1.Range, queryTemplate *template.Template, querySelectors *QuerySelectors) (*float64, v1.Warnings, error) {
	// set up query context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // TODO: fix constant
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
		fmt.Printf("Warnings: %v\n", warnings)
	}

	fmt.Printf("Application %v:%v: query %q:\n\t%T : %v\n\n", app.Metadata.Namespace, app.Metadata.Workload, query, result, result)

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
	value := avg(values...) // prepared for other aggregations

	return &value, warnings, nil
}

func collectDeploymentDetails(promApi v1.API, ctx context.Context, app *appmodel.App, timeRange v1.Range) (v1.Warnings, error) {
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
	res, warnings, err := getAggregateMetric(promApi, ctx, app, timeRange, "kube_pod_spec_volumes_persistentvolumeclaims_readonly", "min")
	if err != nil {
		fmt.Printf("Error querying Prometheus for volume access %v: %v\n", app.Metadata, err)
	} else {
		if len(warnings) > 0 {
			allWarnings = append(allWarnings, warnings...)
			fmt.Printf("Warnings during volume info collection: %v\n", warnings)
		}
		if res != nil && *res == 0 {
			app.Settings.WriteableVolume = true
		}
	}

	// collect replicas
	replicas, warnings, err := getRangedMetric(promApi, ctx, app, timeRange, replicaCountTemplate, &selectors)
	if err != nil {
		fmt.Printf("Error querying Prometheus for replica count %v: %v\n", app.Metadata, err)
	} else {
		if len(warnings) > 0 {
			allWarnings = append(allWarnings, warnings...)
			fmt.Printf("Warnings during replica counts: %v\n", warnings)
		}
		if replicas != nil {
			app.Metrics.AverageReplicas = *replicas
		}
	}

	// collect usage
	cpu_used, warnings, err := getRangedMetric(promApi, ctx, app, timeRange, cpuUtilizationTemplate, &selectors)
	if err != nil {
		fmt.Printf("Error querying Prometheus for CPU utilization %v: %v\n", app.Metadata, err)
	} else {
		if len(warnings) > 0 {
			allWarnings = append(allWarnings, warnings...)
			fmt.Printf("Warnings during cpu utilization collection: %v\n", warnings)
		}
		if cpu_used != nil {
			app.Metrics.CpuUtilization = *cpu_used
		}
	}
	memory_used, warnings, err := getRangedMetric(promApi, ctx, app, timeRange, memoryUtilizationTemplate, &selectors)
	if err != nil {
		fmt.Printf("Error querying Prometheus for memory utilization %v: %v\n", app.Metadata, err)
	} else {
		if len(warnings) > 0 {
			allWarnings = append(allWarnings, warnings...)
			fmt.Printf("Warnings during memory utilization collection: %v\n", warnings)
		}
		if memory_used != nil {
			app.Metrics.MemoryUtilization = *memory_used
		}
	}

	return allWarnings, nil
}

func mapNamespace(promApi v1.API, ctx context.Context, namespace model.LabelValue, timeRange v1.Range) (apps []*appmodel.App) {
	apps = []*appmodel.App{}

	// set up query context with timeout
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second) // TODO: fix constant
	defer cancel()

	// Collect values
	// TODO: consider santizing namespace value despite using %q and model.LabelValue
	result, warnings, err := promApi.Query(reqCtx, fmt.Sprintf("kube_deployment_labels{namespace=%q}", namespace), timeRange.End)
	if err != nil {
		fmt.Print(fmt.Errorf("Error querying Prometheus for namespace %q: %v\n", namespace, err))
		return
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}

	// Collect deployments
	samples, ok := result.(model.Vector)
	if !ok {
		fmt.Printf("Unexpected deployment query result: got type %T, expected Vector", result)
		return
	}
	for _, w := range samples {
		workload := w.Metric["deployment"]
		apps = append(apps, &appmodel.App{Metadata: appmodel.AppMetadata{Namespace: string(namespace), Workload: string(workload), WorkloadKind: "Deployment", WorkloadApiVersion: "apps/v1"}})
		//fmt.Printf("namespace %q: deployment %q\n", namespace, workload)
	}

	// Fill in deployment details
	for _, app := range apps {
		warnings, err := collectDeploymentDetails(promApi, ctx, app, timeRange)
		if len(warnings) > 0 {
		}
		if err != nil {
			fmt.Printf("Failed to collect deployment details for app %v: %v\n", app.Metadata, err)
			app.Opportunity.Cons = append(app.Opportunity.Cons, fmt.Sprintf("Failed to collect deployment details: %v", err))
		}

		// Analyze apps
		if app.Settings.WriteableVolume {
			app.Opportunity.Rating = 0
			app.Opportunity.Confidence = 100
			app.Opportunity.Cons = append(app.Opportunity.Cons, "Stateful: pods have writeable volumes")
		}
		//fmt.Printf("%#v\n\n", app)
	}

	return apps
}

func Init() {
	initializeTemplates()
}

func PromGetAll(promUri *url.URL) ([]*appmodel.App, error) {
	// set up API client
	promApi, err := createAPI(promUri)
	if err != nil {
		return nil, err
	}

	// choose a time range
	now := time.Now()
	timeRange := v1.Range{
		Start: now.Add(-time.Hour * 24 * 7),
		End:   now,
		Step:  time.Hour * 24, //TODO: TBD
	}

	// Collect namespaces
	namespaces, warnings, err := collectNamespaces(promApi, timeRange)
	if err != nil {
		return nil, fmt.Errorf("Error querying Prometheus: %v\n", err)
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}
	fmt.Printf("Namespaces:\n%v\n", namespaces)

	// In parallel, collect the workloads in each namespace
	ctx := context.Background() // TODO - improve, add useful context values
	lists := make(chan []*appmodel.App)
	var wg sync.WaitGroup
	fmt.Printf("Discovered %v namespaces\n", len(namespaces))
	wg.Add(len(namespaces))
	for _, n := range namespaces {
		go func(namespace model.LabelValue) {
			defer wg.Done()
			lists <- mapNamespace(promApi, ctx, namespace, timeRange)
		}(n)
	}
	finalList := make(chan []*appmodel.App)
	go func() {
		apps := make([]*appmodel.App, 0)
		for list := range lists {
			for _, app := range list {
				//fmt.Printf("Reduced app %v:%v\n", app.Metadata.Namespace, app.Metadata.Workload)
				apps = append(apps, app)
			}
		}
		finalList <- apps
	}()
	wg.Wait()
	close(lists)
	apps := []*appmodel.App{}
	for _, app := range <-finalList {
		apps = append(apps, app)
		//fmt.Printf("Found app %v:%v\n", app.Metadata.Namespace, app.Metadata.Workload)
	}
	close(finalList)

	return apps, nil
}
