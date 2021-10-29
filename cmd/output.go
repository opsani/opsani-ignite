/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package cmd

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	appmodel "opsani-ignite/app/model"
)

type AppTable struct {
	wr   io.Writer
	t    tablewriter.Table // table writer, if used
	i    interactiveState  // interactive app root, if used
	yaml *yaml.Encoder     // yaml encoder, if used
}

type DisplayMethods struct {
	WriteHeader func(table *AppTable)
	WriteApp    func(table *AppTable, app *appmodel.App)
	WriteOut    func(table *AppTable)
}

func getDisplayMethods() map[string]DisplayMethods {
	return map[string]DisplayMethods{
		OUTPUT_INTERACTIVE: {(*AppTable).outputInteractiveInit, (*AppTable).outputInteractiveAddApp, (*AppTable).outputInteractiveRun},
		OUTPUT_TABLE:       {(*AppTable).outputTableHeader, (*AppTable).outputTableApp, (*AppTable).outputAnyTableOut},
		OUTPUT_DETAIL:      {(*AppTable).outputDetailHeader, (*AppTable).outputDetailApp, (*AppTable).outputAnyTableOut},
		OUTPUT_YAML:        {(*AppTable).outputYamlHeader, (*AppTable).outputYamlApp, (*AppTable).outputYamlOut},
		OUTPUT_SERVO:       {(*AppTable).outputYamlHeader, (*AppTable).outputServoYamlApp, (*AppTable).outputYamlOut},
	}
}

const (
	colorNone = iota
	colorGreen
	colorYellow
	colorRed
	colorCyan
	colorOrange
)

const (
	alignLeft = iota
	alignCenter
	alignRight
)

func tablewriterColor(color int) int {
	m := map[int]int{
		colorNone:   0,
		colorGreen:  tablewriter.FgGreenColor,
		colorRed:    tablewriter.FgRedColor,
		colorYellow: tablewriter.FgYellowColor,
		colorCyan:   tablewriter.FgCyanColor,
		colorOrange: tablewriter.FgHiYellowColor,
	}
	return m[color]
}

func tablewriterAlign(align int) int {
	m := map[int]int{
		alignLeft:   tablewriter.ALIGN_LEFT,
		alignCenter: tablewriter.ALIGN_CENTER,
		alignRight:  tablewriter.ALIGN_RIGHT,
	}
	return m[align]
}

func appOpportunityAndColor(app *appmodel.App) (oppty string, color int) {
	// list opportunities (usually one but allow for multiple)
	if len(app.Analysis.Opportunities) > 0 {
		oppty = strings.Join(app.Analysis.Opportunities, "\n")
	} else {
		oppty = "n/a"
	}

	// choose color depending on rating
	if !isQualifiedApp(app) {
		color = colorNone // keep default color (neutral)
	} else if app.Analysis.Rating >= 50 {
		color = colorGreen
	} else {
		color = colorYellow
	}

	return
}

func appEfficiencyColor(app *appmodel.App) (color int) {
	rate := app.Analysis.EfficiencyRate

	color = colorNone
	if rate == nil {
		return
	}
	if *rate >= 90 {
		color = colorRed
	} else if *rate >= 60 {
		color = colorGreen
	} else if *rate >= 30 {
		color = colorYellow
	} else {
		color = colorYellow //colorOrange
	}
	return
}

func appTableColor(app *appmodel.App) (color int) {
	var risk appmodel.RiskLevel
	if app.Analysis.ReliabilityRisk != nil {
		risk = *app.Analysis.ReliabilityRisk
	} else {
		risk = appmodel.RISK_UNKNOWN
	}

	// choose color depending on efficiency and risk
	if risk == appmodel.RISK_MEDIUM {
		color = colorYellow
	} else if risk > appmodel.RISK_MEDIUM {
		color = colorRed
	} else if app.Analysis.EfficiencyRate != nil && *app.Analysis.EfficiencyRate >= 60 {
		color = colorGreen
	} else {
		color = colorYellow
	}

	return
}

func flagsString(flags map[appmodel.AppFlag]bool) (ret string) {
	type flagStruct struct {
		flag  appmodel.AppFlag
		value bool
	}

	list := make([]flagStruct, 0, len(flags))
	for f, v := range flags {
		list = append(list, flagStruct{f, v})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].flag < list[j].flag
	})
	for _, e := range list {
		if e.value {
			ret += strings.ToUpper(e.flag.String())
		} else {
			ret += strings.ToLower(e.flag.String())
		}
	}
	return
}

func riskColor(r *appmodel.RiskLevel) int {
	color := colorNone // neutral
	if r == nil {
		return color
	}
	switch *r {
	case appmodel.RISK_LOW:
		color = colorGreen
	case appmodel.RISK_MEDIUM:
		color = colorYellow
	case appmodel.RISK_HIGH:
		color = colorRed
	}
	return color
}

func conclusionColor(c appmodel.AnalysisConclusion) int {
	return map[appmodel.AnalysisConclusion]int{
		appmodel.CONCLUSION_INSUFFICIENT_DATA: colorNone,
		appmodel.CONCLUSION_RELIABILITY_RISK:  colorRed,
		appmodel.CONCLUSION_EXCESSIVE_COST:    colorYellow,
		appmodel.CONCLUSION_OK:                colorGreen,
	}[c]
}

type HeaderInfo struct {
	Title     string
	Alignment int
}

func getHeadersInfo() []HeaderInfo {
	return []HeaderInfo{
		{"Namespace", alignLeft},
		{"Deployment", alignLeft},
		{"Efficiency\nRate", alignRight},
		{"Reliability\nRisk", alignCenter},
		{"Replicas", alignRight},
		{"CPU", alignRight},
		{"Mem", alignRight},
		{"Analysis", alignLeft},
	}
}

func (table *AppTable) outputTableHeader() {
	var headers []string
	var alignments []int
	for _, header := range getHeadersInfo() {
		headers = append(headers, header.Title)
		alignments = append(alignments, tablewriterAlign(header.Alignment))
	}
	table.t.SetHeader(headers)
	table.t.SetColumnAlignment(alignments)
	table.t.SetFooter([]string{})
	table.t.SetCenterSeparator("")
	table.t.SetColumnSeparator("")
	table.t.SetRowSeparator("")
	table.t.SetHeaderLine(false)
	table.t.SetBorder(false)
}

func (table *AppTable) outputTableApp(app *appmodel.App) {
	color := appTableColor(app)
	rowValues := []string{
		app.Metadata.Namespace,
		app.Metadata.Workload,
		fmt.Sprintf("%3v", appmodel.Rate2String(app.Analysis.EfficiencyRate)),
		fmt.Sprintf("%v", appmodel.Risk2String(app.Analysis.ReliabilityRisk)),
		fmt.Sprintf("%.0f", app.Metrics.AverageReplicas),
		fmt.Sprintf("%.0f%%", app.Metrics.CpuUtilization),
		fmt.Sprintf("%.0f%%", app.Metrics.MemoryUtilization),
		app.Analysis.Conclusion.String(),
	}
	cellColors := []int{tablewriterColor(color)}
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
	entries := buildDetailEntries(app)
	for _, e := range entries {
		var twColor []tablewriter.Colors = nil
		if e.Color != colorNone {
			twColor = []tablewriter.Colors{[]int{0}, []int{tablewriterColor(e.Color)}}
		}
		table.t.Rich([]string{e.Name, e.Value}, twColor)
	}
	table.t.Rich([]string{""}, nil)
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
	return &AppTable{wr, *tablewriter.NewWriter(wr), interactiveState{}, nil}
}

type detailEntry struct {
	Name  string
	Value string
	Color int
}

func buildDetailEntries(app *appmodel.App) []detailEntry {
	efficiencyColor := appEfficiencyColor(app)
	opportunityColor := colorGreen
	cautionColor := colorYellow
	blockerColor := colorRed
	riskColor := riskColor(app.Analysis.ReliabilityRisk)
	recommendationColor := colorCyan

	entries := []detailEntry{
		{"Namespace", app.Metadata.Namespace, colorNone},
		{"Deployment", app.Metadata.Workload, colorNone},
		{"Kind", fmt.Sprintf("%v (%v)", app.Metadata.WorkloadKind, app.Metadata.WorkloadApiVersion), colorNone},
		{"Main Container", app.Analysis.MainContainer, colorNone},
		{"Pod QoS Class", app.Settings.QosClass, colorNone},
		{"Average Replica Count", fmt.Sprintf("%3.1f", app.Metrics.AverageReplicas), colorNone},
		{"Container Count", fmt.Sprintf("%3d", len(app.Containers)), colorNone},
		{"CPU Utilization", fmt.Sprintf("%3.0f%%", app.Metrics.CpuUtilization), colorNone},
		{"Memory Utilization", fmt.Sprintf("%3.0f%%", app.Metrics.MemoryUtilization), colorNone},
		{"Network Traffic (approx.)", fmt.Sprintf("%3.1f req/sec", app.Metrics.RequestRate), colorNone},
		{"Opsani Flags", flagsString(app.Analysis.Flags), colorNone},
		{"", "", colorNone},
		{"Efficiency Rate", fmt.Sprintf("%4v%%", appmodel.Rate2String(app.Analysis.EfficiencyRate)), efficiencyColor},
		{"Reliability Risk", fmt.Sprintf("%v", appmodel.Risk2String(app.Analysis.ReliabilityRisk)), riskColor},
		{"Analysis", app.Analysis.Conclusion.String(), conclusionColor(app.Analysis.Conclusion)},
		{"", "", colorNone},
	}

	if len(app.Analysis.Opportunities) > 0 {
		entries = append(entries, detailEntry{"Opportunities", strings.Join(app.Analysis.Opportunities, "\n"), opportunityColor})
	}
	if len(app.Analysis.Cautions) > 0 {
		entries = append(entries, detailEntry{"Cautions", strings.Join(app.Analysis.Cautions, "\n"), cautionColor})
	}
	if len(app.Analysis.Blockers) > 0 {
		entries = append(entries, detailEntry{"Blockers", strings.Join(app.Analysis.Blockers, "\n"), blockerColor})
	}
	if len(app.Analysis.Recommendations) > 0 {
		entries = append(entries, detailEntry{"Recommendations", strings.Join(app.Analysis.Recommendations, "\n"), recommendationColor})
	}

	return entries
}
