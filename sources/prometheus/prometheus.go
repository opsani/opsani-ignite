/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package prometheus

import (
	"context"
	"fmt"
	"sync"

	//"net/http"
	"net/url"
	//"os"
	"time"

	//"github.com/prometheus/common/config"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	appmodel "opsani-ignite/app/model"
)

// Example API usage: https://github.com/prometheus/client_golang/blob/master/api/prometheus/v1/example_test.go
// See also: https://stackoverflow.com/questions/63471775/extract-prometheus-metrics-in-go

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

func collectDeploymentDetails(promApi v1.API, ctx context.Context, app *appmodel.App, timeRange v1.Range) (v1.Warnings, error) {
	// vol_read_only, warnings, err := getAggregateMetric(promApi, ctx, app, timeRange, "kube_pod_spec_volume_persistentvolumeclaims_readonly", "max")
	// if err != nil {
	// fmt.Printf("Error querying Prometheus for volume access %v: %v\n", app.Metadata, err)
	// } else {
	// if len(warnings) > 0 {
	// fmt.Printf("Warnings: %v\n", warnings)
	// }
	// if vol_read_only
	// }
	return nil, nil
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
	}

	return apps
}

func PromGetAll(promUri *url.URL) (*string, error) {
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
	for _, app := range <-finalList {
		fmt.Printf("Found app %v:%v\n", app.Metadata.Namespace, app.Metadata.Workload)
	}
	close(finalList)

	//now := time.Now()
	//lastWeek := now.Add(-time.Hour * 24 * 7)
	r := v1.Range{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
		Step:  time.Minute,
	}

	// set up query context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // TODO: fix constant
	defer cancel()

	// Collect values
	result, warnings, err := promApi.QueryRange(ctx, "rate(prometheus_tsdb_head_samples_appended_total[5m])", r)
	if err != nil {
		return nil, fmt.Errorf("Error querying Prometheus: %v\n", err)
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}
	//fmt.Printf("Result:\n%v\n", result)
	s := result.String()
	return &s, nil
}
