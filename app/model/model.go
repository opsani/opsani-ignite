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
	Replicas        int    `yaml:"-"`
	HpaEnabled      bool   `yaml:"-"`
	VpaEnabled      bool   `yaml:"-"`
	MpaEnabled      bool   `yaml:"-"`
	HpaMinReplicas  int    `yaml:"-"`
	HpaMaxReplicas  int    `yaml:"-"`
	WriteableVolume bool   `yaml:"writeable_volume"`
	QosClass        string `yaml:"qos_class"`
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
	Name string `yaml:"name"`
	Cpu  struct {
		AppContainerResourceInfo `yaml:"resource"`
		SecondsThrottled         float64 `yaml:"seconds_throttled"` // average rate across instances/time
		Shares                   float64 `yaml:"shares"`            // alt source for Cpu.Request, in CPU shares (1000-1024 per core)
	} `yaml:"cpu"`
	Memory struct {
		AppContainerResourceInfo `yaml:"resource"`
	} `yaml:"memory"`
	RestartCount float64 `yaml:"restart_count"` // yaml: don't omit empty, since 0 is a valid value
	PseudoCost   float64 `yaml:"pseudo_cost"`
}

type AppMetrics struct {
	AverageReplicas   float64 `yaml:"average_replicas"`
	CpuUtilization    float64 `yaml:"cpu_saturation"` // aka Saturation, %
	MemoryUtilization float64 `yaml:"memory_saturation"`
	// TODO: add network traffic, esp. indication of traffic
}

type AppOpportunity struct {
	Rating        int      `yaml:"rating"`         // how suitable for optimization
	Confidence    int      `yaml:"confidence"`     // how confident is the rating
	MainContainer string   `yaml:"main_container"` // container to optimize or empty if not identified
	Pros          []string `yaml:"pros"`           // list of pros for optimization
	Cons          []string `yaml:"cons"`           // list of cons for optimizatoin
}

type App struct {
	Metadata    AppMetadata    `yaml:"metadata"`
	Settings    AppSettings    `yaml:"settings"`
	Containers  []AppContainer `yaml:"containers"`
	Metrics     AppMetrics     `yaml:"metrics"`
	Opportunity AppOpportunity `yaml:"analysis"`
}

// Utility methods

func (app *App) ContainerIndexByName(name string) (index int, ok bool) {
	ok = false
	for index = range app.Containers {
		if app.Containers[index].Name == name {
			ok = true
			return
		}
	}
	return
}
