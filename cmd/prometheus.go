/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package cmd

import (
	"context"
	"fmt"
	//"net/http"
	"net/url"
	//"os"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	//"github.com/prometheus/common/config"
)

// Example API usage: https://github.com/prometheus/client_golang/blob/master/api/prometheus/v1/example_test.go

func createAPI(uri *url.URL) (v1.API, error) {
	client, err := api.NewClient(api.Config{
		Address: uri.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating Prometheus client: %v\n", err)
	}

	return v1.NewAPI(client), nil
}

func promGetAll() (*string, error) {
	promApi, err := createAPI(promUri)
	if err != nil {
		return nil, err
	}

	r := v1.Range{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
		Step:  time.Minute,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // TODO: fix constant
	defer cancel()
	
	result, warnings, err := promApi.QueryRange(ctx, "rate(prometheus_tsdb_head_samples_appended_total[5m])", r)
	if err != nil {
		return nil, fmt.Errorf("Error querying Prometheus: %v\n", err)
	}
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}
	fmt.Printf("Result:\n%v\n", result)
	s := result.String()
	return &s, nil
}