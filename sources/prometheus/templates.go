package prometheus

import (
	"text/template"
)

var replicaCountTemplate *template.Template
var containerRestartsTemplate *template.Template
var cpuUtilizationTemplate *template.Template
var memoryUtilizationTemplate *template.Template
var containerInfoTemplate *template.Template
var containerResourceRequestsTemplate *template.Template
var containerResourceLimitsTemplate *template.Template
var containerCpuUseTemplate *template.Template
var containerMemoryUseTemplate *template.Template
var containerCpuSaturationTemplate *template.Template
var containerMemorySaturationTemplate *template.Template
var containerCpuSecondsThrottledTemplate *template.Template
var containerRxPacketsTemplate *template.Template
var containerTxPacketsTemplate *template.Template

// Useful References:
//
// CPU and memory saturation references:
//   https://blog.freshtracks.io/a-deep-dive-into-kubernetes-metrics-part-3-container-resource-metrics-361c5ee46e66
//   https://github.com/google/cadvisor/issues/2026
//
// Join example:
//   https://ypereirareis.github.io/blog/2020/02/21/how-to-join-prometheus-metrics-by-label-with-promql/
// CPU throttling / CFS info
//   https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/6/html/resource_management_guide/sec-cpu

func initializeTemplates() {
	// replica count (averaged over the range)
	replicaCountTemplate = template.Must(template.New("prometheusPodAverageReplicas").Parse(
		`kube_deployment_status_replicas{namespace="{{ .Namespace }}", deployment="{{ .Workload }}"}`))

	// container restarts
	containerRestartsTemplate = template.Must(template.New("prometheusRestartsTemplate").Parse(
		`avg by (container) (kube_pod_container_status_restarts_total{ {{ .PodSelector }} })`))

	// old style, pod-aggregated (but may be less precise)
	cpuUtilizationTemplate = template.Must(template.New("prometheusPodCpuUtilization").Parse(
		`avg(sum by (pod, container) (rate(container_cpu_usage_seconds_total{ {{ .PodSelector }} }[60s]) * 1024 * 60) / on (pod, container) (container_spec_cpu_shares{ {{ .PodSelector }} }) / 60 * 100)`))
	memoryUtilizationTemplate = template.Must(template.New("prometheusPodMemoryUtilization").Parse(
		`avg(container_memory_working_set_bytes{ {{ .PodSelector }} } / (1024 * 1024))`))

	// container info & settings
	containerInfoTemplate = template.Must(template.New("prometheusContainerInfo").Parse(
		`sum by (container) (kube_pod_container_info{ {{ .PodSelector }} })`))
	containerResourceRequestsTemplate = template.Must(template.New("prometheusContainerResourceRequests").Parse(
		`avg by (container, resource) (kube_pod_container_resource_requests{ {{ .PodSelector }} })`))
	containerResourceLimitsTemplate = template.Must(template.New("prometheusContainerResourceLimits").Parse(
		`avg by (container, resource) (kube_pod_container_resource_limits{ {{ .PodSelector }} })`))

	// container use
	containerCpuUseTemplate = template.Must(template.New("prometheusContainerCpuUseTemplate").Parse(
		`avg by (container) (rate(container_cpu_usage_seconds_total{ {{ .PodSelector }} }[5m]))`))
	containerMemoryUseTemplate = template.Must(template.New("prometheusContainerMemoryUseTemplate").Parse(
		`avg by (container) (container_memory_working_set_bytes{ {{ .PodSelector }} })`))

	// container utilization
	containerCpuSaturationTemplate = template.Must(template.New("prometheusContainerCpuSaturationTemplate").Parse(
		`avg (rate(container_cpu_usage_seconds_total{ {{ .PodSelector }},container!~"|POD" }[5m]) / on(pod, container) 
			kube_pod_container_resource_requests{  {{ .PodSelector }},resource="cpu"}) by (container)`))
	containerMemorySaturationTemplate = template.Must(template.New("prometheusContainerMemorySaturationTemplate").Parse(
		`avg (container_memory_working_set_bytes{ {{ .PodSelector }},container!~"|POD" } / on(pod, container) 
			kube_pod_container_resource_requests{  {{ .PodSelector }},resource="memory"}) by (container)`))

	// container CPU-specifics
	containerCpuSecondsThrottledTemplate = template.Must(template.New("prometheusContainerCpuSecondsThrottledTemplate").Parse(
		`avg by (container) (rate(container_cpu_cfs_throttled_seconds_total{ {{ .PodSelector }} }[5m]))`))

	// container Memory-specifics
	// TODO (e.g., oom kill count, maybe from kube_pod_container_status_terminated_reason)

	// container network traffic
	// note: network stats are per pod (container="POD"), not per container
	containerRxPacketsTemplate = template.Must(template.New("prometheusContainerRxPacketsTemplate").Parse(
		`avg (rate(container_network_receive_packets_total{ {{ .PodSelector }} }[5m]))`))
	containerTxPacketsTemplate = template.Must(template.New("prometheusContainerTxPacketsTemplate").Parse(
		`avg (rate(container_network_transmit_packets_total{ {{ .PodSelector }} }[5m]))`))

}
