/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package cmd

import (
	"fmt"
	"io"
	appmodel "opsani-ignite/app/model"
	"strings"

	"github.com/olekukonko/tablewriter"
)

type AppTable struct {
	tablewriter.Table // allows for adding methods locally
}

type DisplayMethods struct {
	WriteHeader func(table *AppTable)
	WriteApp    func(table *AppTable, app *appmodel.App)
}

func getDisplayMethods() map[string]DisplayMethods {
	return map[string]DisplayMethods{
		OUTPUT_TABLE:  {(*AppTable).outputTableHeader, (*AppTable).outputTableApp},
		OUTPUT_DETAIL: {(*AppTable).outputDetailHeader, (*AppTable).outputDetailApp},
	}
}

func appReasonAndColor(app *appmodel.App) (string, int) {
	var reason string
	var color int // tablewriter colors

	// handle unqualified apps
	if !isQualifiedApp(app) {
		if len(app.Opportunity.Cons) > 0 {
			reason = app.Opportunity.Cons[0]
		} else {
			reason = "n/a"
		}
		color = 0 // keep default color (neutral); alt: tablewriter.FgRedColor

		return reason, color
	}

	// handle qualified apps depending on rating
	if app.Opportunity.Rating >= 50 {
		if len(app.Opportunity.Pros) > 0 {
			reason = app.Opportunity.Pros[0]
		} else {
			reason = "n/a"
		}
		color = tablewriter.FgGreenColor
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

func (table *AppTable) outputTableHeader() {
	const RIGHT = tablewriter.ALIGN_RIGHT
	const LEFT = tablewriter.ALIGN_LEFT

	table.SetHeader([]string{"Rating", "Confidence", "Namespace", "Deployment", "Replicas", "CPU", "Mem", "Reason"})
	table.SetColumnAlignment([]int{RIGHT, RIGHT, LEFT, LEFT, RIGHT, RIGHT, RIGHT, LEFT})
	table.SetFooter([]string{})
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
}

func (table *AppTable) outputTableApp(app *appmodel.App) {
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

func (table *AppTable) outputDetailHeader() {
	table.SetCenterSeparator("")
	table.SetColumnSeparator(":")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
}

func (table *AppTable) outputDetailApp(app *appmodel.App) {
	blank := []string{""}
	_, appColor := appReasonAndColor(app)
	appColors := []tablewriter.Colors{[]int{0}, []int{appColor}}
	prosColors := []tablewriter.Colors{[]int{0}, []int{tablewriter.FgGreenColor}}
	consColors := []tablewriter.Colors{[]int{0}, []int{tablewriter.FgYellowColor}}
	if app.Opportunity.Rating < 0 {
		consColors = []tablewriter.Colors{[]int{0}, []int{tablewriter.FgRedColor}}
	}

	table.Rich([]string{"Namespace", app.Metadata.Namespace}, nil)
	table.Rich([]string{"Deployment", app.Metadata.Workload}, nil)
	table.Rich([]string{"Kind", fmt.Sprintf("%v (%v)", app.Metadata.WorkloadKind, app.Metadata.WorkloadApiVersion)}, nil)

	table.Rich([]string{"Rating", fmt.Sprintf("%3d%%", app.Opportunity.Rating)}, appColors)
	table.Rich([]string{"Confidence", fmt.Sprintf("%3d%%", app.Opportunity.Confidence)}, appColors)

	//table.Rich(blank, nil)
	if len(app.Opportunity.Pros) > 0 {
		table.Rich([]string{"Pros", strings.Join(app.Opportunity.Pros, "\n")}, prosColors)
	}
	if len(app.Opportunity.Cons) > 0 {
		table.Rich([]string{"Cons", strings.Join(app.Opportunity.Cons, "\n")}, consColors)
	}

	//table.Rich(blank, nil)
	table.Rich([]string{"Average Replica Count", fmt.Sprintf("%3.0f%%", app.Metrics.AverageReplicas)}, nil)
	table.Rich([]string{"CPU Utilization", fmt.Sprintf("%3.0f%%", app.Metrics.CpuUtilization)}, nil)
	table.Rich([]string{"Memory Utilization", fmt.Sprintf("%3.0f%%", app.Metrics.MemoryUtilization)}, nil)

	table.Rich(blank, nil)
}

func newAppTable(wr io.Writer) *AppTable {
	return &AppTable{*tablewriter.NewWriter(wr)}
}
