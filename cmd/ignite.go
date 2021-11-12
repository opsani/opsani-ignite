/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	appmodel "opsani-ignite/app/model"

	"opsani-ignite/log"
	prom "opsani-ignite/sources/prometheus"
)

const LOG_FILE = "opsani-ignite.log"

func opportunitySorter(apps []*appmodel.App, i, j int) bool {
	ia, ja := apps[i], apps[j]
	// sort by rating first
	if ia.Analysis.Rating > ja.Analysis.Rating {
		return true
	}
	if ia.Analysis.Rating < ja.Analysis.Rating {
		return false
	}
	// same rating, top confidence first for + rated apps; top confidence at bottom for - rated apps
	if ia.Analysis.Confidence > ja.Analysis.Confidence {
		return ia.Analysis.Rating >= 0
	}
	if ia.Analysis.Confidence < ja.Analysis.Confidence {
		return ia.Analysis.Rating < 0
	}
	// withint the same ratings & confidence, order by namespace, workload name alphabetically
	// (we do this so that the order is stable and the order is user-friendly)
	if ia.Metadata.Namespace < ja.Metadata.Namespace {
		return true
	}
	if ia.Metadata.Namespace < ja.Metadata.Namespace {
		return false
	}
	return ia.Metadata.Workload < ja.Metadata.Workload
}

func isQualifiedApp(app *appmodel.App) bool {
	return app.Analysis.Rating >= 0
}

func displayConfig(namespace, deployment string) {
	msgs := make([]string, 0)

	msgs = append(msgs, fmt.Sprintf("Using Prometheus API at %q", promUri))

	anzMsg := "Analyzing "
	if namespace != "" {
		if deployment != "" {
			anzMsg += fmt.Sprintf("namespace %v, deployment %v", namespace, deployment)
		} else {
			anzMsg += fmt.Sprintf("all deployments in namespace %v", namespace)
		}
	} else {
		anzMsg += "all deployments in all non-system namespaces"
	}
	msgs = append(msgs, anzMsg)

	msgs = append(msgs, fmt.Sprintf("From %v to %v in increments of %v.",
		timeStart.Format(time.RFC3339), timeEnd.Format(time.RFC3339), timeStep))

	for _, msg := range msgs {
		log.Print(msg)
		fmt.Fprintln(os.Stderr, msg)
	}
	fmt.Fprintln(os.Stderr, "")
}

func displayResults(apps []*appmodel.App, targetedApps bool) {
	// auto-enable show-all-apps in case no apps meet requirements
	if targetedApps {
		hideBlocked = false // ignore hideBlocked when namespace+deployment are explicitly specified
	} else if hideBlocked {
		qualified := 0
		for _, app := range apps {
			if isQualifiedApp(app) {
				qualified += 1
			}
		}
		if qualified == 0 {
			hideBlocked = false
			log.Infof("No applications meet optimization prerequisites. Showing all applications")
			fmt.Fprintf(os.Stderr, "No applications meet optimization prerequisites. Showing all applications\n")
		}
	}

	// build table & display (stream, yaml or interactive)
	table := newAppTable(os.Stdout)
	skipped := 0
	display := getDisplayMethods()[outputFormat]
	display.WriteHeader(table)
	for _, app := range apps {
		if hideBlocked && !isQualifiedApp(app) {
			skipped += 1
			continue
		}
		display.WriteApp(table, app)
	}
	display.WriteOut(table)
	if skipped > 0 {
		log.Infof("%v applications were not shown as they don't meet optimization prerequisites", skipped)
		fmt.Fprintf(os.Stderr, "%v applications were not shown as they don't meet optimization prerequisites. Remove the --hide-blocked option to see all apps\n", skipped)
	}
}

func runIgnite(cmd *cobra.Command, args []string) {
	logFile, err := os.OpenFile(LOG_FILE, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetupLogLevel(showDebug, suppressWarnings)

	// determine namespace & deployment selection
	namespace := ""
	deployment := ""
	if len(args) >= 1 {
		namespace = args[0]
	}
	if len(args) >= 2 {
		deployment = args[1]
	}
	displayConfig(namespace, deployment) // and API url and time range/step

	// Create root context
	ctx := context.Background()

	// get applications from the cluster
	prom.Init()
	apps := make([]*appmodel.App, 0)
	err = log.GoWithProgress(func (progressCallback log.ProgressUpdateFunc) error {
		var innerErr error
		apps, innerErr = prom.PromGetAll(ctx, promUri, namespace, deployment, "apps/v1", "Deployment", timeStart, timeEnd, timeStep, progressCallback)
		return innerErr
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to obtain data from Prometheus at %q: %v", promUri, err)
		os.Exit(1)
	}
	if len(apps) == 0 {
		if deployment == "" {
			fmt.Fprintf(os.Stderr, "No applications found. Try specifying explicit namespace and, optionally, deployment to analyze")
		} else {
			fmt.Fprintf(os.Stderr, "Application %q not found in namespace %q", deployment, namespace)
		}
		return
	}

	// analyze apps, assign rating and confidence (updates in place)
	for _, app := range apps {
		analyzeApp(app)
	}

	// sort table by opportunity
	sort.Slice(apps, func(i, j int) bool {
		return opportunitySorter(apps, i, j)
	})

	// display results
	displayResults(apps, deployment != "")

	fmt.Fprint(os.Stderr, "To optimize your application, sign up for a free trial account at https://opsani.com/create-your-account2/#ignite\n")
}
