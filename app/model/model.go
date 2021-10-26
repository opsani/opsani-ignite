package model

import "fmt"

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
	AverageReplicas     float64 `yaml:"average_replicas"`      // averaged over the evaluated time range
	CpuUtilization      float64 `yaml:"cpu_saturation"`        // aka Saturation, in percent, can be 0 or >100
	MemoryUtilization   float64 `yaml:"memory_saturation"`     // aka Saturation, in percent, can be 0 or >100
	CpuSecondsThrottled float64 `yaml:"cpu_seconds_throttled"` // sum of seconds throttled/second across all containers
	PacketReceiveRate   float64 `yaml:"packet_receive_rate"`   // per second
	PacketTransmitRate  float64 `yaml:"packet_transmit_rate"`  // per second
	RequestRate         float64 `yaml:"request_rate"`          // per second
}

type AppFlag int

const (
	F_MAIN_CONTAINER = iota
	F_MULTI_CONTAINER
	F_WRITEABLE_VOLUME
	F_RESOURCE_SPEC
	F_RESOURCE_LIMITS
	F_RESOURCE_GUARANTEED
	F_UTILIZATION
	F_BURST
	F_TRAFFIC
	F_SINGLE_REPLICA
	F_MANY_REPLICAS
)

func (f AppFlag) String() string {
	return []string{"C", "I", "W", "R", "L", "G", "U", "B", "T", "S", "M"}[f]
}

func (f AppFlag) MarshalYAML() (interface{}, error) {
	return f.String(), nil
}

type RiskLevel int

const (
	RISK_UNKNOWN = iota
	RISK_NONE
	RISK_LOW
	RISK_MEDIUM
	RISK_HIGH
	RISK_CRITICAL
)

func (r *RiskLevel) SafeRiskLevel() RiskLevel {
	if r == nil {
		return RISK_UNKNOWN
	}
	return *r
}

func (r RiskLevel) String() string {
	return []string{"-", "None", "Low", "Medium", "High", "Critical"}[r]
}

func (r RiskLevel) MarshalYAML() (interface{}, error) {
	return r.String(), nil
}

type AnalysisConclusion int

const (
	CONCLUSION_INSUFFICIENT_DATA = iota
	CONCLUSION_RELIABILITY_RISK
	CONCLUSION_EXCESSIVE_COST
	CONCLUSION_OK
)

func (c AnalysisConclusion) String() string {
	return []string{"(insufficient data)", "Reliability Risk", "Excessive Cost", "Look good!"}[c]
}

func (c AnalysisConclusion) MarshalYAML() (interface{}, error) {
	return c.String(), nil
}

type AppAnalysis struct {
	Rating          int                `yaml:"rating"`           // how suitable for optimization
	Confidence      int                `yaml:"confidence"`       // how confident is the rating
	MainContainer   string             `yaml:"main_container"`   // container to optimize or empty if not identified
	EfficiencyRate  *int               `yaml:"efficiency_rate"`  // 0-100%
	ReliabilityRisk *RiskLevel         `yaml:"reliability_risk"` // high/medium/low
	Conclusion      AnalysisConclusion `yaml:"conclusion"`       // analysis conclusion
	Flags           map[AppFlag]bool   `yaml:"flags"`            // flags
	Opportunities   []string           `yaml:"opportunities"`    // list of optimization opportunities
	Cautions        []string           `yaml:"cautions"`         // list of concerns/cautions
	Blockers        []string           `yaml:"blockers"`         // list of blockers prevention optimization
	Recommendations []string           `yaml:"recommendations"`  // list of recommendations for improvement
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

func Rate2String(s *int) string {
	if s == nil {
		return "n/a"
	}
	return fmt.Sprintf("%v", *s)
}
func Risk2String(r *RiskLevel) string {
	if r == nil {
		return "n/a"
	}
	return r.String()
}
