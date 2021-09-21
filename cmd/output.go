/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package cmd

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	appmodel "opsani-ignite/app/model"
)

type AppTable struct {
	wr   io.Writer
	t    tablewriter.Table // table writer, if used
	yaml *yaml.Encoder     // yaml encoder, if used
}

type DisplayMethods struct {
	WriteHeader func(table *AppTable)
	WriteApp    func(table *AppTable, app *appmodel.App)
	WriteOut    func(table *AppTable)
}

func getDisplayMethods() map[string]DisplayMethods {
	return map[string]DisplayMethods{
		OUTPUT_TABLE:  {(*AppTable).outputTableHeader, (*AppTable).outputTableApp, (*AppTable).outputAnyTableOut},
		OUTPUT_DETAIL: {(*AppTable).outputDetailHeader, (*AppTable).outputDetailApp, (*AppTable).outputAnyTableOut},
		OUTPUT_YAML:   {(*AppTable).outputYamlHeader, (*AppTable).outputYamlApp, (*AppTable).outputYamlOut},
		OUTPUT_SERVO:  {(*AppTable).outputYamlHeader, (*AppTable).outputServoYamlApp, (*AppTable).outputYamlOut},
	}
}

func appReasonAndColor(app *appmodel.App) (string, int) {
	var reason string
	var color int // tablewriter colors

	// handle unqualified apps
	if !isQualifiedApp(app) {
		if len(app.Analysis.Cons) > 0 {
			reason = app.Analysis.Cons[0]
		} else {
			reason = "n/a"
		}
		color = 0 // keep default color (neutral); alt: tablewriter.FgRedColor

		return reason, color
	}

	// handle qualified apps depending on rating
	if app.Analysis.Rating >= 50 {
		if len(app.Analysis.Pros) > 0 {
			reason = app.Analysis.Pros[0]
		} else {
			reason = "n/a"
		}
		color = tablewriter.FgGreenColor
	} else {
		if len(app.Analysis.Cons) > 0 {
			reason = app.Analysis.Cons[0]
		} else if len(app.Analysis.Pros) > 0 {
			reason = app.Analysis.Pros[0]
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

	table.t.SetHeader([]string{"Rating", "Confidence", "Namespace", "Deployment", "Container", "Instances", "CPU", "Mem", "Reason"})
	table.t.SetColumnAlignment([]int{RIGHT, RIGHT, LEFT, LEFT, LEFT, RIGHT, RIGHT, RIGHT, LEFT})
	table.t.SetFooter([]string{})
	table.t.SetCenterSeparator("")
	table.t.SetColumnSeparator("")
	table.t.SetRowSeparator("")
	table.t.SetHeaderLine(false)
	table.t.SetBorder(false)
}

func (table *AppTable) outputTableApp(app *appmodel.App) {
	reason, color := appReasonAndColor(app)
	rowValues := []string{
		fmt.Sprintf("%d%%", app.Analysis.Rating),
		fmt.Sprintf("%d%%", app.Analysis.Confidence),
		app.Metadata.Namespace,
		app.Metadata.Workload,
		app.Analysis.MainContainer,
		fmt.Sprintf("%.0fx%d", app.Metrics.AverageReplicas, len(app.Containers)),
		fmt.Sprintf("%.0f%%", app.Metrics.CpuUtilization),
		fmt.Sprintf("%.0f%%", app.Metrics.MemoryUtilization),
		reason,
	}
	cellColors := []int{color}
	rowColors := make([]tablewriter.Colors, len(rowValues))
	for i := range rowColors {
		rowColors[i] = cellColors
	}
	table.t.Rich(rowValues, rowColors)
}

func (table *AppTable) outputDetailHeader() {
	table.t.SetCenterSeparator("")
	table.t.SetColumnSeparator(":")
	table.t.SetRowSeparator("")
	table.t.SetHeaderLine(false)
	table.t.SetBorder(false)
	table.t.SetAlignment(tablewriter.ALIGN_LEFT)
}

func (table *AppTable) outputDetailApp(app *appmodel.App) {
	blank := []string{""}
	_, appColor := appReasonAndColor(app)
	appColors := []tablewriter.Colors{[]int{0}, []int{appColor}}
	prosColors := []tablewriter.Colors{[]int{0}, []int{tablewriter.FgGreenColor}}
	consColors := []tablewriter.Colors{[]int{0}, []int{tablewriter.FgYellowColor}}
	if app.Analysis.Rating < 0 {
		consColors = []tablewriter.Colors{[]int{0}, []int{tablewriter.FgRedColor}}
	}

	table.t.Rich([]string{"Namespace", app.Metadata.Namespace}, nil)
	table.t.Rich([]string{"Deployment", app.Metadata.Workload}, nil)
	table.t.Rich([]string{"Kind", fmt.Sprintf("%v (%v)", app.Metadata.WorkloadKind, app.Metadata.WorkloadApiVersion)}, nil)
	table.t.Rich([]string{"Main Container", app.Analysis.MainContainer}, nil)

	table.t.Rich([]string{"Rating", fmt.Sprintf("%4d%%", app.Analysis.Rating)}, appColors)
	table.t.Rich([]string{"Confidence", fmt.Sprintf("%4d%%", app.Analysis.Confidence)}, appColors)

	//table.Rich(blank, nil)
	if len(app.Analysis.Pros) > 0 {
		table.t.Rich([]string{"Pros", strings.Join(app.Analysis.Pros, "\n")}, prosColors)
	}
	if len(app.Analysis.Cons) > 0 {
		table.t.Rich([]string{"Cons", strings.Join(app.Analysis.Cons, "\n")}, consColors)
	}

	//table.Rich(blank, nil)
	table.t.Rich([]string{"Average Replica Count", fmt.Sprintf("%3.0f%%", app.Metrics.AverageReplicas)}, nil)
	table.t.Rich([]string{"Container Count", fmt.Sprintf("%3d", len(app.Containers))}, nil)
	table.t.Rich([]string{"CPU Utilization", fmt.Sprintf("%3.0f%%", app.Metrics.CpuUtilization)}, nil)
	table.t.Rich([]string{"Memory Utilization", fmt.Sprintf("%3.0f%%", app.Metrics.MemoryUtilization)}, nil)

	table.t.Rich(blank, nil)
}

func (table *AppTable) outputAnyTableOut() {
	fmt.Println("")
	table.t.Render()
	fmt.Println("")
}

func (table *AppTable) outputYamlHeader() {
	table.yaml = yaml.NewEncoder(table.wr)
}

func (table *AppTable) outputYamlApp(app *appmodel.App) {
	err := table.yaml.Encode(*app)
	if err != nil {
		log.Errorf("Failed to write app %v to yaml: %v", app.Metadata, err)
	}
}

func (table *AppTable) outputYamlOut() {
	table.yaml.Close()
}

func alignedResourceValue(v float64, up bool) float64 {
	const step = 0.125

	// take care of edge cases
	if v <= step {
		if v < step {
			log.Warnf("resource value %v is less than the step %v; assuming step", v, step)
		}
		return step
	}

	// calculate alignment
	n := math.Floor(v / 0.125)
	if n*step == v {
		return v // aligned already
	}

	// align up or down, as requested
	if up {
		n += 1
	}
	return n * step
}

func selectResourceValue(r *appmodel.AppContainerResourceInfo) float64 {
	if r.Request > 0 {
		return r.Request
	}
	if r.Limit > 0 {
		return r.Limit
	}
	if r.Usage > 0 {
		return r.Usage
	}
	return 0
}

func (table *AppTable) outputServoYamlApp(app *appmodel.App) {
	type ResourceRange struct {
		Min string `yaml:"min,omitempty"`
		Max string `yaml:"max,omitempty"`
	}
	type OpsaniDev struct {
		Namespace  string
		Deployment string
		Container  string
		Service    string        `yaml:"service,omitempty"`
		Cpu        ResourceRange `yaml:"cpu,omitempty"`
		Memory     ResourceRange `yaml:"memory,omitempty"`
	}

	if app.Analysis.MainContainer == "" {
		log.Errorf("Cannot produce servo.yaml output for application %v: no main container identified", app.Metadata)
		return
	}
	cIndex, ok := app.ContainerIndexByName(app.Analysis.MainContainer)
	if !ok { // shouldn't happen
		log.Errorf("Cannot produce servo.yaml output for application %v: main container %q not found", app.Metadata, app.Analysis.MainContainer)
		return
	}

	var opsaniDev OpsaniDev
	opsaniDev.Namespace = app.Metadata.Namespace
	opsaniDev.Deployment = app.Metadata.Workload
	opsaniDev.Container = app.Analysis.MainContainer
	opsaniDev.Service = app.Metadata.Workload // TODO: get the real service, this is a stub

	c := &app.Containers[cIndex]
	cpuCores := selectResourceValue(&c.Cpu.AppContainerResourceInfo)
	opsaniDev.Cpu.Min = fmt.Sprintf("%g", alignedResourceValue(cpuCores/4.0, false))
	opsaniDev.Cpu.Max = fmt.Sprintf("%g", alignedResourceValue(cpuCores*2.0, true))
	memGib := selectResourceValue(&c.Cpu.AppContainerResourceInfo)
	opsaniDev.Memory.Min = fmt.Sprintf("%gGi", alignedResourceValue(memGib/4.0, false))
	opsaniDev.Memory.Max = fmt.Sprintf("%gGi", alignedResourceValue(memGib*2.0, true))

	configRoot := make(map[string]OpsaniDev, 1)
	configRoot["opsani_dev"] = opsaniDev
	configRootBuf, err := yaml.Marshal(configRoot)
	if err != nil {
		log.Errorf("Failed to marshal %#v to yaml: %v", configRoot, err)
		return
	}

	servoYaml := make(map[string]string, 1)
	servoYaml["servo.yaml"] = string(configRootBuf)

	if err := table.yaml.Encode(servoYaml); err != nil {
		log.Errorf("Failed to write app %v to yaml: %v", app.Metadata, err)
	}
}

func newAppTable(wr io.Writer) *AppTable {
	return &AppTable{wr, *tablewriter.NewWriter(wr), nil}
}

/*
servo.yaml: |
opsani_dev:
{%- raw %}
  namespace: {{ NAMESPACE }}
  deployment: {{ DEPLOYMENT }}
  container: {{ CONTAINER }}
  service: {{ SERVICE }}
{% endraw %}
  cpu:
	min: 250m
	max: '3.0'
  memory:
	min: 128.0MiB
	max: 3.0GiB
*/
