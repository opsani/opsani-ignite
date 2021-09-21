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

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	appmodel "opsani-ignite/app/model"

	prom "opsani-ignite/sources/prometheus"
)

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

func runIgnite(cmd *cobra.Command, args []string) {
	if showDebug {
		log.SetLevel(log.TraceLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	log.Printf("Using Prometheus API at %q\n", promUri)

	// Create root context
	ctx := context.Background()

	// get applications from the cluster
	prom.Init()
	apps, err := prom.PromGetAll(ctx, promUri, namespace, deployment, "apps/v1", "Deployment")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to obtain data from Prometheus at %q: %v", promUri, err)
		os.Exit(1)
	}
	if len(apps) == 0 {
		if deployment == "" {
			fmt.Fprintf(os.Stderr, "No applications found. Try specifying explicit --namespace and, optionally, --deployment to analyze")
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

	// --- Display applications in a table
	// for _, app := range apps {
	// 	fmt.Printf("%#v\n\n", app)
	// }

	// auto-enable show-all-apps in case no apps meet requirements
	if !showAllApps {
		qualified := 0
		for _, app := range apps {
			if isQualifiedApp(app) {
				qualified += 1
			}
		}
		if qualified == 0 && deployment == "" { // if a deployment is specified, it will be shown anyway
			showAllApps = true
			log.Infof("No highly rated applications found. Showing all applications")
		}
	}

	// display results
	table := newAppTable(os.Stdout)
	skipped := 0
	display := getDisplayMethods()[outputFormat]
	display.WriteHeader(table)
	for _, app := range apps {
		// skip unqualified apps (unless either -a flag or explicitly identified app)
		if !isQualifiedApp(app) && !showAllApps && deployment == "" {
			skipped += 1
			continue
		}
		display.WriteApp(table, app)
	}
	display.WriteOut(table)
	if skipped > 0 {
		log.Infof("%v applications were not shown due to low rating. Use --show-all to see all apps", skipped)
	}

}
