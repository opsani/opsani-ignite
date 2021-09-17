package model

type AppMetadata struct {
	//Cluster string // needed?
	//Name               string
	Namespace          string
	Workload           string
	WorkloadKind       string
	WorkloadApiVersion string
	//Labels []string  // needed?
}

type AppSettings struct {
	Replicas        int
	HpaEnabled      bool
	VpaEnabled      bool
	MpaEnabled      bool
	HpaMinReplicas  int
	HpaMaxReplicas  int
	WriteableVolume bool
	// TODO: consider adding replicas stats (min/max/avg/median)
}

type AppContainerResourceInfo struct {
	Unit       string // unit for resource's Request, Limit and Usage
	Request    float64
	Limit      float64
	Usage      float64
	Saturation float64 // Usage/Request if Request!=0; otherise Usage/Limit if Limit!=0; otherwise 0 (ratio, not percent)
}

type AppContainer struct {
	Name string
	Cpu  struct {
		AppContainerResourceInfo
		SecondsThrottled float64 // average rate across instances/time
		Shares           float64 // alt source for Cpu.Request, in CPU shares (1000-1024 per core)
	}
	Memory struct {
		AppContainerResourceInfo
	}
	RestartCount float64
}

type AppMetrics struct {
	AverageReplicas   float64
	CpuUtilization    float64 // aka Saturation, %
	MemoryUtilization float64
	// TODO: add network traffic, esp. indication of traffic
}

type AppOpportunity struct {
	Rating     int      // how suitable for optimization
	Confidence int      // how confident is the rating
	Pros       []string // list of pros for optimization
	Cons       []string // list of cons for optimizatoin
}

type App struct {
	Metadata    AppMetadata
	Settings    AppSettings
	Containers  []AppContainer
	Metrics     AppMetrics
	Opportunity AppOpportunity
}
