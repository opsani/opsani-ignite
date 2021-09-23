package model

const (
	QOS_GUARANTEED = "guaranteed"
	QOS_BURSTABLE  = "burstable"
	QOS_BESTEFFORT = "besteffort"
)

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
	AverageReplicas    float64 `yaml:"average_replicas"`     // averaged over the evaluated time range
	CpuUtilization     float64 `yaml:"cpu_saturation"`       // aka Saturation, in percent, can be 0 or >100
	MemoryUtilization  float64 `yaml:"memory_saturation"`    // aka Saturation, in percent, can be 0 or >100
	PacketReceiveRate  float64 `yaml:"packet_receive_rate"`  // per second
	PacketTransmitRate float64 `yaml:"packet_transmit_rate"` // per second
	RequestRate        float64 `yaml:"request_rate"`         // per second
}

type AppFlag int

const (
	F_WRITEABLE_VOLUME = iota
	F_RESOURCE_SPEC
	F_SINGLE_REPLICA
	F_MANY_REPLICAS
	F_TRAFFIC
	F_UTILIZATION
	F_BURST
	F_MAIN_CONTAINER
)

func (f AppFlag) String() string {
	return []string{"V", "R", "S", "M", "T", "U", "B", "C"}[f]
}

type AppAnalysis struct {
	Rating           int              `yaml:"rating"`         // how suitable for optimization
	Confidence       int              `yaml:"confidence"`     // how confident is the rating
	MainContainer    string           `yaml:"main_container"` // container to optimize or empty if not identified
	EfficiencyScore  int              `yaml:"efficiency_score"`
	ReliabilityScore int              `yaml:"reliability_score,omitempty"`
	PerformanceScore int              `yaml:"performance_score,omitempty"`
	Flags            map[AppFlag]bool `yaml:"flags"`         // flags
	Opportunities    []string         `yaml:"opportunities"` // list of optimization opportunities
	Cautions         []string         `yaml:"cautions"`      // list of concerns/cautions
	Blockers         []string         `yaml:"blockers"`      // list of blockers prevention optimization
}

type App struct {
	Metadata   AppMetadata    `yaml:"metadata"`
	Settings   AppSettings    `yaml:"settings"`
	Containers []AppContainer `yaml:"containers"`
	Metrics    AppMetrics     `yaml:"metrics"`
	Analysis   AppAnalysis    `yaml:"analysis"`
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
