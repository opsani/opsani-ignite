# Opsani Ignite for Kubernetes

[![Release](https://img.shields.io/github/release/opsani/opsani-ignite.svg?style=for-the-badge)](https://github.com/opsani/opsani-ignite/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/opsani/opsani-ignite?style=for-the-badge)](https://goreportcard.com/report/github.com/opsani/opsani-ignite)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=for-the-badge)](LICENSE)
[![stability-alpha](https://img.shields.io/badge/stability-alpha-f4d03f.svg?style=for-the-badge)](https://github.com/mkenney/software-guides/blob/master/STABILITY-BADGES.md#alpha)
[![Github All Releases](https://img.shields.io/github/downloads/opsani/opsani-ignite/total.svg?style=for-the-badge)](https://github.com/opsani/opsani-ignite/releases/latest)

Opsani Ignite analyzes applications running on a Kubernetes cluster in order to 
identify _performance and reliability risks_, as well as _inefficient configurations_. It then identifies specific _corrective actions_ that align the application's configuration with deployment best practices for production environments and may also reduce the application's resource footprint.

CAUTION: Opsani Ignite is a new tool, still in alpha. We appreciate [feedback and suggestions](#feedback-and-suggestions).

# Download and Install Ignite

To install `opsani-ignite`, download the binary for your OS (macOS, Linux or Windows) from the [latest release](https://github.com/opsani/opsani-ignite/releases/latest) and place it somewhere along your shell's path. Check back often as we release updated analysis capabilities frequently; if your version is more than a week old, please see if a newer version is available before using it.

# Run Ignite

To run Opsani Ignite, you will first want to set up port forwarding to the Prometheus API on your cluster. A typical command looks like this (assuming your Prometheus is called `prometheus-server` and runs in the `prometheus` namespace):

`kubectl port-forward service/prometheus-server 9090:80 -n prometheus`

Once port forwarding is active, run the `opsani-ignite` executable, providing the URL to the port-forwarded Prometheus API:

`opsani-ignite -p http://localhost:9090`

Opsani Ignite has three phases: discovery, analysis and recommendations.

## Phase 1: Discovery

On startup, Ignite discovers the applications running on the Kubernetes cluster. By querying your Prometheus monitoring system, Ignite finds all non-system namespaces and the deployment workloads running in them; it then obtain their key settings and metrics. 

![discovery](docs/discovery.png)

By default, Ignite looks at the last 7 days of metrics for each application to capture most daily and weekly load and performance variations.

## Phase 2: Analysis

Ignite analyzes each application, looking at pods and containers that make up the application in order to uncover specific omissions of best practices for reliable production deployments. It looks at important characteristics such as the pod's quality of service (QoS), replica count, resource allocation, usage, limits, and processed load. Ignite then identifies areas requiring attention that are either causing or can cause performance and reliability issues.

![analysis](docs/analysis.png)

In addition, Ignite determines whether the application is overprovisioned resulting in a higher than necessary cloud spend. In such cases, it also estimates the likely savings that can be obtained through optimization.

## Phase 3: Recommendations

When an application is selected (pressing Enter in the table of apps), Ignite produces a set of actionable recommendations for improving the efficiency, performance, and reliability of the application. The recommendations fall into several categories, including production best practices (for example, setting resource requests and limits), as well as optimal and resilient operation optimization recommendations. Applying these recommendations results in improved performance and efficiency, as well as increased resilience of their applications under load.

![recommendations](docs/recommendations.png)

# Interactive... Stdout... or YAML output

By default, Ignite is text-based interactive tool (using the fantastic [tview](https://github.com/rivo/tview) package, familiar to those who use the equally magnificent [k9s](https://github.com/derailed/k9s) tool). Ignite's command line options can change the output to simple stdout text view and even full-detail YAML output that can be used to integrate Ignite into your dashboards and higher level tools.

# Optimization Recommendations

Opsani Ignite provides analysis and a number of additional recommendations to improve performance, reliability and efficiency. 

Best practices require correctly setting resource requirements in a way that meets the performance and reliability requirements of an application (typically, latency and error rate service level objectives), while using assigned resources efficiently to control cloud costs. These values can be discovered manually, often through an onerous and repetitive manual tuning process. 

They can also be automatically identified using automatic optimization services, such as the Opsani optimization-as-a-service tool. Those who are interested in how continuous optimization can remediate these issues can go to the [Opsani website](https://opsani.com), set up a [free trial account](https://opsani.com/create-your-account2/#ignite-readme) and attach the optimizer to their application. Connecting an application to the optimizer typically takes 10-15 minutes and, in a few hours, produces concrete, tested resource specifications that can be applied using a simple `kubectl` command.


# Feedback and Suggestions

The Ignite tool is the result of analyzing thousands of applications as part of our work at Opsani. We released it as an open source tool in order to share our experience and learning with the Kubernetes community and help improve application reliability and efficiency. The source code is available to review and to contribute.

We will appreciate your feedback. Please send us a few lines about your experience--or, even better--a screenshot with the results (be they good or not so good) at ![support](docs/support.png).

# Troubleshooting 

Opsani Ignite records diagnostic information in `opsani-ignite.log`. You can increase the logging level by adding the `--debug` option to the command line.

# Where To Get Help

You can reach out to Opsani Technical support at ![support](docs/support.png) or, for faster response, use the chat bot on the [Opsani web site](https://www.opsani.com).
