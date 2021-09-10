/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package cmd

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	prom "opsani-ignite/sources/prometheus"
)

func run_ignite(cmd *cobra.Command, args []string) {
	fmt.Printf("Getting Prometheus metrics from %q\n", promUri)

	prom.Init()
	apps, err := prom.PromGetAll(promUri)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// --- Display applications in a table
	// for _, app := range apps {
	// 	fmt.Printf("%#v\n\n", app)
	// }

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Rating %", "Confidence %", "Namespace", "Deployment", "Replicas", "CPU %", "Mem %", "Reason"})

	for _, app := range apps {
		var reason string
		var color int //TODO: tablewriter colors
		if app.Opportunity.Rating > 50 {
			if len(app.Opportunity.Pros) > 0 {
				reason = app.Opportunity.Pros[0]
			} else {
				reason = "n/a"
			}
			color = tablewriter.FgHiGreenColor
		} else if app.Opportunity.Rating == 0 {
			if len(app.Opportunity.Cons) > 0 {
				reason = app.Opportunity.Cons[0]
			} else {
				reason = "n/a"
			}
			color = tablewriter.FgRedColor
		} else {
			if len(app.Opportunity.Cons) > 0 {
				reason = app.Opportunity.Cons[0]
			} else if len(app.Opportunity.Pros) > 0 {
				reason = app.Opportunity.Pros[0]
			} else {
				reason = "n/a"
			}
			color = tablewriter.FgYellowColor
		}
		rowValues := []string{
			fmt.Sprintf("%d", app.Opportunity.Rating),
			fmt.Sprintf("%d", app.Opportunity.Confidence),
			app.Metadata.Namespace,
			app.Metadata.Workload,
			fmt.Sprintf("%.0f", app.Metrics.AverageReplicas),
			fmt.Sprintf("%.0f", app.Metrics.CpuUtilization),
			fmt.Sprintf("%.0f", app.Metrics.MemoryUtilization),
			reason,
		}
		cellColors := []int{color}
		rowColors := make([]tablewriter.Colors, len(rowValues))
		for i := range rowColors {
			rowColors[i] = cellColors
		}
		table.Rich(rowValues, rowColors)
	}
	table.SetFooter([]string{})
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.Render()
}
