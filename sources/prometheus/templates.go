package prometheus

import (
	"text/template"
)

var replicaCountTemplate *template.Template
var cpuUtilizationTemplate *template.Template
var memoryUtilizationTemplate *template.Template

func initializeTemplates() {
	replicaCountTemplate = template.Must(template.New("prometheusPodAverageReplicas").Parse(
		`kube_deployment_status_replicas{deployment="{{ .Workload }}",namespace="{{ .Namespace }}"}`))
	cpuUtilizationTemplate = template.Must(template.New("prometheusPodCpuUtilization").Parse(
		`avg(sum by (pod, container) (rate(container_cpu_usage_seconds_total{ {{ .PodSelector }} }[60s]) * 1024 * 60) / on (pod, container) (container_spec_cpu_shares{ {{ .PodSelector }} }) / 60 * 100)`))
	memoryUtilizationTemplate = template.Must(template.New("prometheusPodMemoryUtilization").Parse(
		`avg(container_memory_working_set_bytes{ {{ .PodSelector }} } / (1024 * 1024))`))
}
