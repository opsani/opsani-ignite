/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package cmd

import (
	"fmt"
	"math"
	"sort"

	log "github.com/sirupsen/logrus"

	appmodel "opsani-ignite/app/model"
	opsmath "opsani-ignite/math"
)

const (
	CPU_WEIGHT = 0.6
	MEM_WEIGHT = 0.4
)

type ResourceUtilizationRating struct {
	UtilizationFloor float64
	RatingBump       int
}

// const table
func getResourceUtilizationRatingsTable() []ResourceUtilizationRating {
	return []ResourceUtilizationRating{
		{100, 30}, // >=100 provides opportunity to improve performance/rightsize
		{80, 10},  // 80..100 likely not much room to optimize
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

// --- Container-level Analysis ----------------------------------------------

func calcSaturation(r *appmodel.AppContainerResourceInfo, app *appmodel.App, container string, resource string) float64 {
	var base float64

	// determine resource base. If no Request value but Limit value is set, use Limit (TODO if correct)
	if r.Request > 0 {
		base = r.Request
	} else {
		base = r.Limit
	}
	if base == 0 {
		return r.Saturation // in case it's already retrieved
	}

	// compute and check
	sat := r.Usage / base
	if r.Saturation != 0 { // in case it's already retrieved
		if math.Abs(sat-r.Saturation)/r.Saturation > 0.1 { // div0 not possible; 0.1 is heuristical
			log.Warnf("Calculated %v saturation and retrieved saturation differ significantly for app %v container %v (%v!=%v)",
				resource, app.Metadata, container, sat, r.Saturation)
		}
		sat = r.Saturation // TODO: TBD which one to keep/trust more
	}

	// round it to meaningful numbers (no need for MagicRound, keep decimals only if close to 0% but not quite)
	// i.e., this SHOULD NOT make saturation 0 if it isn't; but OK to round it otherwise
	// nb: it is OK/possible for saturation to exceed 100%
	if sat > 0.01 {
		satRounded := float64(math.Round(sat*100)) / 100
		if satRounded < 0.01 {
			// should never happen,but using belt and suspenders to not mess up
			log.Warnf("unexpected math result in resource rounding (%v->%v); keeping original value", sat, satRounded)
			// keep original value in sat
		} else {
			// use rounded value
			sat = satRounded
		}
	}

	return sat
}

func containerResourceCostingValue(r *appmodel.AppContainerResourceInfo) float64 {
	if r.Usage > 0 {
		if r.Request > 0 && r.Usage > r.Request {
			return r.Usage
		}
		return r.Usage
	}
	if r.Request > 0 {
		return r.Request
	}
	return r.Limit
}

func identifyMainContainer(app *appmodel.App) string {
	// handle trivial cases
	if len(app.Containers) < 1 {
		return ""
	}
	if len(app.Containers) == 1 {
		return app.Containers[0].Name
	}

	// use name-based heuristics
	namesMap := make(map[string]bool)
	for _, c := range app.Containers {
		namesMap[c.Name] = true
	}
	if _, ok := namesMap["main"]; ok {
		return "main"
	}
	if _, ok := namesMap[app.Metadata.Workload]; ok { // TODO: consider heuristic for xxx-deployment (extract xxx)
		return app.Metadata.Workload
	}

	// use heuristics based on container's size/use
	// nb: assume containers are already sorted by pseudo cost
	if app.Containers[0].PseudoCost > app.Containers[1].PseudoCost {
		return app.Containers[0].Name
	}
	// TODO: add more heuristics (e.g., based on cpu/mem, usage, etc.)

	log.Warnf("Could not identify application's main container for %v", app)
	return ""
}

func analyzeContainers(app *appmodel.App) {
	// Calculate container resource saturation (utilization)
	for i := range app.Containers {
		c := &app.Containers[i]
		c.Cpu.Saturation = calcSaturation(&c.Cpu.AppContainerResourceInfo, app, c.Name, "CPU")
		c.Memory.Saturation = calcSaturation(&c.Memory.AppContainerResourceInfo, app, c.Name, "Memory")
	}

	// Calculate container pseudo cost
	for i := range app.Containers {
		c := &app.Containers[i]
		cores := containerResourceCostingValue(&c.Cpu.AppContainerResourceInfo)
		gib := float64(containerResourceCostingValue(&c.Memory.AppContainerResourceInfo)) / float64(1024*1024)
		c.PseudoCost = opsmath.MagicRound(cores*0.0175 + gib*0.0125)
	}

	// sort containers info
	sort.Slice(app.Containers, func(i, j int) bool {
		return app.Containers[i].PseudoCost < app.Containers[i].PseudoCost
	})

	// identify main container (if possible)
	if app.Analysis.MainContainer == "" {
		app.Analysis.MainContainer = identifyMainContainer(app)
	}

	// identify QoS
	// see https://kubernetes.io/docs/tasks/configure-pod-container/quality-service-pod/#create-a-pod-that-gets-assigned-a-qos-class-of-guaranteed
	//     https://www.replex.io/blog/everything-you-need-to-know-about-kubernetes-quality-of-service-qos-classes (somewhat imprecise)
	// TODO

	// Calculate pod resource saturation (utilization)
	// TODO
}

func computePodQoS(app *appmodel.App) string {
	// following the rules at https://kubernetes.io/docs/tasks/configure-pod-container/quality-service-pod/
	// note: ignoring the init containers
	// TODO: use other sources to get the QoS, use this one to (a) populate QoS if no QoS info found and
	//       (b) validate the QoS found (e.g., degrade)

	allMatch := true  // assume all containers, all resources match the guaranteed requirements
	oneMatch := false // track if at least one container, one resource has resources specified
	for i := range app.Containers {
		c := &app.Containers[i]

		if c.Cpu.Limit > 0 {
			if c.Cpu.Request > 0 && c.Cpu.Request != c.Cpu.Limit {
				allMatch = false
			}
			oneMatch = true
		} else if c.Cpu.Request > 0 {
			allMatch = false
			oneMatch = true
		} else {
			allMatch = false
		}
		if c.Memory.Limit > 0 {
			if c.Memory.Request > 0 && c.Memory.Request != c.Memory.Limit {
				allMatch = false
			}
			oneMatch = true
		} else if c.Memory.Request > 0 {
			allMatch = false
			oneMatch = true
		} else {
			allMatch = false
		}
	}

	if allMatch {
		return appmodel.QOS_GUARANTEED
	} else if oneMatch {
		return appmodel.QOS_BURSTABLE
	} else {
		return appmodel.QOS_BESTEFFORT
	}
}

func resourcesExplicitlyDefined(app *appmodel.App) (bool, string) {
	// select the main container
	if app.Analysis.MainContainer == "" {
		return false, "main container not identified"
	}
	var main *appmodel.AppContainer
	for i := range app.Containers {
		if app.Containers[i].Name == app.Analysis.MainContainer {
			main = &app.Containers[i]
			break
		}
	}
	if main == nil {
		return false, "main container not found" // should never happen
	}

	// check requirements
	cpuGood := main.Cpu.Request > 0 || main.Cpu.Limit > 0
	memGood := main.Memory.Request > 0 || main.Memory.Limit > 0
	if cpuGood && memGood {
		return true, ""
	}

	// construct feedback message for human consumption
	if !cpuGood && !memGood {
		return false, "Resources not specified (request or limit for cpu and memory is required"
	}
	if !cpuGood {
		return false, "CPU resources not specified (request or limit is required)"
	}
	return false, "Memory resources not specified (request or limit required)"
}

// --- App-level Analysis ----------------------------------------------------

func preAnalyzeApp(app *appmodel.App) {
	// analyze container info, aggregating it to application level
	analyzeContainers(app)

	// update app-level resource saturation (based on main container)
	if app.Analysis.MainContainer != "" {
		if index, ok := app.ContainerIndexByName(app.Analysis.MainContainer); ok {
			// replace old-style pod-level saturation with saturation of the target container
			m := &app.Containers[index]
			if m.Cpu.Saturation > 0 {
				app.Metrics.CpuUtilization = m.Cpu.Saturation * 100
			}
			if m.Memory.Saturation > 0 {
				app.Metrics.MemoryUtilization = m.Memory.Saturation * 100
			}
		}
	}

	// validate or determine QoS
	computedQos := computePodQoS(app)
	if app.Settings.QosClass == "" {
		app.Settings.QosClass = computedQos
	} else if app.Settings.QosClass != computedQos {
		log.Warnf("Computed QoS class %q does not match discovered QoS class %q for app %v; assuming the latter",
			computedQos, app.Settings.QosClass, app.Metadata)
	}

	// validate or determine request rate
	// Notes:
	// - packet rate can be used as a proxy to requests per second
	// - bidirectional traffic is required to consider traffic as requests/replies
	computedRps := 0.0
	if app.Metrics.PacketReceiveRate > 0 && app.Metrics.PacketTransmitRate > 0 {
		computedRps = app.Metrics.PacketReceiveRate // packets received ≈ requests
	}
	if app.Metrics.RequestRate == 0 {
		app.Metrics.RequestRate = opsmath.MagicRound(computedRps)
	}

}

func efficiencyImprovementEstimate(app *appmodel.App) string {
	cpu := app.Metrics.CpuUtilization
	mem := app.Metrics.MemoryUtilization
	if cpu == 0 || mem == 0 {
		return ""
	}
	if cpu >= 80 || mem >= 80 {
		return ""
	}
	imp := (160 - cpu - mem) / 2.0
	imp = float64(math.Round(imp/10) * 10)
	if imp >= 60 {
		return fmt.Sprintf("2x-%gx", 1+math.Round(100.0/imp*10)/10)
	} else if imp > 20 {
		return fmt.Sprintf("%0.f-%0.f%%", imp-20, imp)
	} else {
		return fmt.Sprintf("up to %0.f%%", imp)
	}
}

func analyzeApp(app *appmodel.App) {
	// finalize basis and prepare for analysis
	preAnalyzeApp(app)

	// start from current analysis
	o := app.Analysis
	if o.Flags == nil {
		o.Flags = make(map[appmodel.AppFlag]bool)
	}

	// check main container
	if app.Analysis.MainContainer != "" {
		o.Flags[appmodel.F_MAIN_CONTAINER] = true
	} else {
		o.Blockers = append(o.Blockers, "Could not identify main container")
		o.Flags[appmodel.F_MAIN_CONTAINER] = false
	}

	// having a writeable PVC disqualifies the app immediately (stateful)
	if app.Settings.WriteableVolume {
		o.Blockers = append(o.Blockers, "Stateful: pods have writeable volumes")
		o.Flags[appmodel.F_WRITEABLE_VOLUME] = true
	} else {
		o.Flags[appmodel.F_WRITEABLE_VOLUME] = false
	}

	// missing resource specification (main container has no QoS)
	if resGood, msg := resourcesExplicitlyDefined(app); resGood {
		o.Flags[appmodel.F_RESOURCE_SPEC] = true
	} else {
		o.Flags[appmodel.F_RESOURCE_SPEC] = false
		o.Blockers = append(o.Blockers, msg)
	}

	// analyze resource utilization
	o.Flags[appmodel.F_UTILIZATION] = app.Metrics.CpuUtilization > 0 && app.Metrics.MemoryUtilization > 0
	utilBump := utilizationCombinedRating(app.Metrics.CpuUtilization, app.Metrics.MemoryUtilization)
	if utilBump != 0 {
		o.Rating += utilBump
		o.Confidence += 30
		if app.Metrics.CpuUtilization >= 100 || app.Metrics.MemoryUtilization >= 100 {
			o.Opportunities = append(o.Opportunities, "Improve performance/reliability")
			o.Flags[appmodel.F_BURST] = true
		} else if utilBump >= 30 {
			effImpr := efficiencyImprovementEstimate(app)
			if effImpr != "" {
				effImpr = " by " + effImpr
			}
			o.Opportunities = append(o.Opportunities, fmt.Sprintf("Improve efficiency%v", effImpr))
			o.Flags[appmodel.F_BURST] = false
		} else if utilBump == 0 {
			o.Cautions = append(o.Cautions, "Idle application")
			o.Flags[appmodel.F_BURST] = false
		}
	}

	// compute scores
	o.EfficiencyScore = int(math.Round(app.Metrics.CpuUtilization*CPU_WEIGHT + app.Metrics.MemoryUtilization*MEM_WEIGHT))

	// analyze request rate
	if app.Metrics.RequestRate == 0 {
		o.Blockers = append(o.Blockers, "No requests are being processed")
		o.Flags[appmodel.F_TRAFFIC] = false
	} else if app.Metrics.RequestRate < 2 {
		o.Cautions = append(o.Cautions, "Low request rate")
		o.Rating -= 10
		// note: don't set traffic flag
	} else {
		o.Flags[appmodel.F_TRAFFIC] = true
		if app.Metrics.RequestRate > 100 {
			o.Rating += 10 // low confidence as we don't know if traffic is served or originated
		}
	}

	// analyze replica count
	if app.Metrics.AverageReplicas <= 1 {
		o.Rating -= 20
		o.Confidence += 10
		o.Cautions = append(o.Cautions, "Less than 2 replicas")
		o.Flags[appmodel.F_SINGLE_REPLICA] = true
		o.Flags[appmodel.F_MANY_REPLICAS] = false
	} else if app.Metrics.AverageReplicas >= 7 {
		o.Rating += 20
		o.Confidence += 30
		o.Flags[appmodel.F_SINGLE_REPLICA] = false
		o.Flags[appmodel.F_MANY_REPLICAS] = true
	} else if app.Metrics.AverageReplicas >= 3 {
		o.Rating += 10
		o.Confidence += 10
		o.Flags[appmodel.F_SINGLE_REPLICA] = false
		o.Flags[appmodel.F_MANY_REPLICAS] = false
	}

	// finalize blockers
	if len(o.Blockers) > 0 {
		o.Rating = -100
		o.Confidence = 100
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
	app.Analysis = o
}
