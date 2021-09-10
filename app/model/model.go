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

type AppMetrics struct {
	AverageReplicas   float64
	CpuUtilization    float64
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
	Metrics     AppMetrics
	Opportunity AppOpportunity
}
