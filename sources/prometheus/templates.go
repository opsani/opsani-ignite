package prometheus

import (
	"text/template"
)

var replicaCountTemplate *template.Template
var cpuUtilizationTemplate *template.Template
var memoryUtilizationTemplate *template.Template
var containerInfoTemplate *template.Template
var containerResourceRequestsTemplate *template.Template
var containerResourceLimitsTemplate *template.Template
var containerCpuUseTemplate *template.Template
var containerMemoryUseTemplate *template.Template

// CPU and memory saturation references:
//   https://blog.freshtracks.io/a-deep-dive-into-kubernetes-metrics-part-3-container-resource-metrics-361c5ee46e66
//   https://github.com/google/cadvisor/issues/2026

func initializeTemplates() {
	replicaCountTemplate = template.Must(template.New("prometheusPodAverageReplicas").Parse(
		`kube_deployment_status_replicas{namespace="{{ .Namespace }}", deployment="{{ .Workload }}"}`))

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

	// container CPU-specifics

	// container Memory-specifics
	// TODO (e.g., oom kill count, maybe from kube_pod_container_status_terminated_reason)

	// container restarts
	containerMemoryUseTemplate = template.Must(template.New("prometheusContainerMemoryUseTemplate").Parse(
		`avg by (container) (kube_pod_container_status_restarts_total{ {{ .PodSelector }} })`))

	/*

				sum(
					rate(container_cpu_usage_seconds_total[5m]))
				by (container_name)

				sum(
					rate(container_cpu_cfs_throttled_seconds_total[5m]))
				by (container_name)
				avg by (container) (rate(container_cpu_cfs_throttled_seconds_total{namespace="bank-of-anthos-opsani",pod=~"frontend-.*"}[5m]))

				container_memory_working_set_bytes

				avg by (container) (container_memory_working_set_bytes{namespace="bank-of-anthos-opsani", pod=~"frontend-.*"})

				sum(container_memory_working_set_bytes) by (container_name) / sum(label_join(kube_pod_container_resource_limits_memory_bytes,
					"container_name", "", "container")) by (container_name)

				// from https://github.com/google/cadvisor/issues/2026
				sum(rate(container_cpu_usage_seconds_total{name!~".*prometheus.*", image!="", container_name!="POD"}[5m])) by (pod_name, container_name) /
		           sum(container_spec_cpu_quota{name!~".*prometheus.*", image!="", container_name!="POD"}/container_spec_cpu_period{name!~".*prometheus.*", image!="", container_name!="POD"}) by (pod_name, container_name)

				// CPU throttling / CFS info
				https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/6/html/resource_management_guide/sec-cpu


	*/
}
