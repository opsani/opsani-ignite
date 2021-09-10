/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	appmodel "opsani-ignite/app/model"

	prom "opsani-ignite/sources/prometheus"
)

type ResourceUtilizationRating struct {
	UtilizationFloor float64
	RatingBump       int
}

// const table
func getResourceUtilizationRatingsTable() []ResourceUtilizationRating {
	return []ResourceUtilizationRating{
		{100, 60}, // >=100 provides opportunity to improve performance/rightsize
		{80, 20},  // 80..100 likely not much room to optimize
		{40, 40},  // 40..80 some optimization room
		{1, 60},   // 1..40 likely lots to optimize
		{0, 0},    // no utilization - likely can't optimize
	}
}

func utilizationRating(v float64) int {
	for _, r := range getResourceUtilizationRatingsTable() {
		if v >= r.UtilizationFloor {
			return r.RatingBump
		}
	}
	return 0
}

func utilizationCombinedRating(cpuUtil, memUtil float64) int {
	// convert resource utilization % to rating bump, for each resource separately
	cpuBump, memBump := utilizationRating(cpuUtil), utilizationRating(memUtil)

	// if rating is 0 for any resource, use 0
	if cpuBump == 0 || memBump == 0 {
		return 0
	}

	// average bump
	return (cpuBump + memBump) / 2
}

func opportunitySorter(apps []*appmodel.App, i, j int) bool {
	ia, ja := apps[i], apps[j]
	// sort by rating first
	if ia.Opportunity.Rating > ja.Opportunity.Rating {
		return true
	}
	if ia.Opportunity.Rating < ja.Opportunity.Rating {
		return false
	}
	// same rating, top confidence first for + rated apps; top confidence at bottom for - rated apps
	if ia.Opportunity.Confidence > ja.Opportunity.Confidence {
		return ia.Opportunity.Rating >= 0
	}
	if ia.Opportunity.Confidence < ja.Opportunity.Confidence {
		return ia.Opportunity.Rating < 0
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

func appReasonAndColor(app *appmodel.App) (string, int) {
	var reason string
	var color int // tablewriter colors

	if app.Opportunity.Rating >= 50 {
		if len(app.Opportunity.Pros) > 0 {
			reason = app.Opportunity.Pros[0]
		} else {
			reason = "n/a"
		}
		color = tablewriter.FgGreenColor
	} else if app.Opportunity.Rating < 0 {
		if len(app.Opportunity.Cons) > 0 {
			reason = app.Opportunity.Cons[0]
		} else {
			reason = "n/a"
		}
		color = tablewriter.FgRedColor // 0 // keep default color (neutral)
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

	return reason, color
}

func analyzeApp(app *appmodel.App) {
	var o appmodel.AppOpportunity

	// having a writeable PVC disqualifies the app immediately (stateful)
	if app.Settings.WriteableVolume {
		o.Rating = -100
		o.Confidence = 100
		o.Cons = append(o.Cons, "Stateful: pods have writeable volumes")
	}

	// analyze utilization
	utilBump := utilizationCombinedRating(app.Metrics.CpuUtilization, app.Metrics.MemoryUtilization)
	if utilBump != 0 {
		o.Rating += utilBump
		o.Confidence += 30
		if utilBump >= 30 {
			o.Pros = append(o.Pros, "Resource utilization")
		} else if utilBump == 0 {
			o.Cons = append(o.Cons, "Idle application")
		}
	}

	// analyze replica count
	if app.Metrics.AverageReplicas <= 1 {
		o.Rating -= 20
		o.Confidence += 10
		o.Cons = append(o.Cons, "Less than 2 replicas")
	} else if app.Metrics.AverageReplicas >= 7 {
		o.Rating += 20
		o.Confidence += 30
		o.Pros = append(o.Pros, "7 or more replicas")
	} else if app.Metrics.AverageReplicas >= 3 {
		o.Rating += 10
		o.Confidence += 10
	}

	// bound rating and confidence
	if o.Rating < -100 {
		o.Rating = -100
	} else if o.Rating > 100 {
		o.Rating = 100
	}
	if o.Confidence < 0 {
		o.Confidence = 0
	} else if o.Confidence > 100 {
		o.Confidence = 100
	}

	// update
	app.Opportunity = o
}

func runIgnite(cmd *cobra.Command, args []string) {
	fmt.Printf("Getting Prometheus metrics from %q\n", promUri)

	prom.Init()
	apps, err := prom.PromGetAll(promUri)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
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

	const RIGHT = tablewriter.ALIGN_RIGHT
	const LEFT = tablewriter.ALIGN_LEFT

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Rating", "Confidence", "Namespace", "Deployment", "Replicas", "CPU", "Mem", "Reason"})
	table.SetColumnAlignment([]int{RIGHT, RIGHT, LEFT, LEFT, RIGHT, RIGHT, RIGHT, LEFT})
	for _, app := range apps {
		reason, color := appReasonAndColor(app)
		rowValues := []string{
			fmt.Sprintf("%d%%", app.Opportunity.Rating),
			fmt.Sprintf("%d%%", app.Opportunity.Confidence),
			app.Metadata.Namespace,
			app.Metadata.Workload,
			fmt.Sprintf("%.0f", app.Metrics.AverageReplicas),
			fmt.Sprintf("%.0f%%", app.Metrics.CpuUtilization),
			fmt.Sprintf("%.0f%%", app.Metrics.MemoryUtilization),
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
