package execution

// ActivityMetricSample is one observation of an activity attempt. CPU and I/O
// counters are cumulative; memory is an instantaneous gauge. A source must
// only report values attributable to the activity's own execution group.
type ActivityMetricSample struct {
	RunID       string  `json:"runId"`
	ActivityID  string  `json:"activityId"`
	Attempt     int     `json:"attempt"`
	ObservedAt  float64 `json:"observedAt"`
	CPUSeconds  float64 `json:"cpuSeconds"`
	MemoryBytes int64   `json:"memoryBytes"`
	ReadBytes   int64   `json:"readBytes"`
	WriteBytes  int64   `json:"writeBytes"`
	Source      string  `json:"source"`
	Scope       string  `json:"scope"`
}

type ActivityMetricSummary struct {
	RunID           string  `json:"runId"`
	ActivityID      string  `json:"activityId"`
	Attempt         int     `json:"attempt"`
	Samples         int     `json:"samples"`
	FirstObservedAt float64 `json:"firstObservedAt"`
	LastObservedAt  float64 `json:"lastObservedAt"`
	CPUSeconds      float64 `json:"cpuSeconds"`
	AverageCPUCores float64 `json:"averageCpuCores"`
	PeakMemoryBytes int64   `json:"peakMemoryBytes"`
	ReadBytes       int64   `json:"readBytes"`
	WriteBytes      int64   `json:"writeBytes"`
	Source          string  `json:"source"`
	Scope           string  `json:"scope"`
}
