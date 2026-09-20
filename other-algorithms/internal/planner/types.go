package planner

type Input struct {
	Workflow         WorkflowVersion           `json:"workflow" yaml:"workflow"`
	ExecutionScope   ExecutionScope            `json:"executionScope" yaml:"executionScope"`
	Resources        []Resource                `json:"resources" yaml:"resources"`
	NetworkTopology  NetworkTopology           `json:"networkTopology" yaml:"networkTopology"`
	ActivityProfiles []ActivityResourceProfile `json:"activityProfiles,omitempty" yaml:"activityProfiles,omitempty"`
	DeadlineSeconds  float64                   `json:"deadlineSeconds,omitempty" yaml:"deadlineSeconds,omitempty"`
	Budget           float64                   `json:"budget,omitempty" yaml:"budget,omitempty"`
}

type WorkflowDefinition struct {
	ID      string          `json:"id" yaml:"id"`
	Version WorkflowVersion `json:"version" yaml:"version"`
}

type WorkflowVersion struct {
	ID               string                   `json:"id" yaml:"id"`
	Activities       []Activity               `json:"activities" yaml:"activities"`
	Dependencies     []ActivityDependency     `json:"dependencies" yaml:"dependencies"`
	DataDependencies []ActivityDataDependency `json:"dataDependencies,omitempty" yaml:"dataDependencies,omitempty"`
}

type Activity struct {
	ID             string            `json:"id" yaml:"id"`
	ActivityTypeID string            `json:"activityTypeId" yaml:"activityTypeId"`
	ExternalID     string            `json:"externalId" yaml:"externalId"`
	Name           string            `json:"name" yaml:"name"`
	Resources      ActivityResources `json:"resources" yaml:"resources"`
	Simulation     *Simulation       `json:"simulation,omitempty" yaml:"simulation,omitempty"`
	Priority       int               `json:"priority" yaml:"priority"`
	Metadata       map[string]any    `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type ActivityResources struct {
	CPU         float64 `json:"cpu" yaml:"cpu"`
	MemoryBytes int64   `json:"memoryBytes" yaml:"memoryBytes"`
}

type Simulation struct {
	DurationSeconds float64 `json:"durationSeconds,omitempty" yaml:"durationSeconds,omitempty"`
	FLOPs           float64 `json:"flops,omitempty" yaml:"flops,omitempty"`
}

type ActivityDependency struct {
	ActivityID          string `json:"activityId" yaml:"activityId"`
	DependsOnActivityID string `json:"dependsOnActivityId" yaml:"dependsOnActivityId"`
}

type ActivityDataDependency struct {
	ProducerActivityID string `json:"producerActivityId" yaml:"producerActivityId"`
	ConsumerActivityID string `json:"consumerActivityId" yaml:"consumerActivityId"`
	SizeBytes          int64  `json:"sizeBytes" yaml:"sizeBytes"`
}

type ActivityResourceProfile struct {
	ActivityTypeID  string  `json:"activityTypeId" yaml:"activityTypeId"`
	ResourceID      string  `json:"resourceId" yaml:"resourceId"`
	RuntimeSeconds  float64 `json:"runtimeSeconds" yaml:"runtimeSeconds"`
	PeakMemoryBytes int64   `json:"peakMemoryBytes" yaml:"peakMemoryBytes"`
}

type ExecutionScope struct {
	ID                    string   `json:"id" yaml:"id"`
	EnvironmentVersionIDs []string `json:"environmentVersionIds" yaml:"environmentVersionIds"`
}

type Resource struct {
	ID                   string         `json:"id" yaml:"id"`
	EnvironmentVersionID string         `json:"environmentVersionId" yaml:"environmentVersionId"`
	ExecutionTarget      string         `json:"executionTarget,omitempty" yaml:"executionTarget,omitempty"`
	Type                 string         `json:"type" yaml:"type"`
	Name                 string         `json:"name" yaml:"name"`
	CPUCores             int            `json:"cpuCores" yaml:"cpuCores"`
	CPUCapacity          float64        `json:"cpuCapacity" yaml:"cpuCapacity"`
	MemoryBytes          int64          `json:"memoryBytes" yaml:"memoryBytes"`
	ComputeSpeedup       float64        `json:"computeSpeedup" yaml:"computeSpeedup"`
	PricePerSecond       float64        `json:"pricePerSecond" yaml:"pricePerSecond"`
	BootOverheadSeconds  float64        `json:"bootOverheadSeconds" yaml:"bootOverheadSeconds"`
	ContainerOverhead    float64        `json:"containerOverheadSeconds" yaml:"containerOverheadSeconds"`
	Schedulable          bool           `json:"schedulable" yaml:"schedulable"`
	Metadata             map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type NetworkTopology struct {
	ID               string        `json:"id" yaml:"id"`
	ExecutionScopeID string        `json:"executionScopeId" yaml:"executionScopeId"`
	Links            []NetworkLink `json:"links" yaml:"links"`
}

type NetworkLink struct {
	SourceResourceID       string  `json:"sourceResourceId" yaml:"sourceResourceId"`
	TargetResourceID       string  `json:"targetResourceId" yaml:"targetResourceId"`
	BandwidthBitsPerSecond float64 `json:"bandwidthBitsPerSecond" yaml:"bandwidthBitsPerSecond"`
	LatencySeconds         float64 `json:"latencySeconds" yaml:"latencySeconds"`
	Bidirectional          bool    `json:"bidirectional" yaml:"bidirectional"`
}

type ImportEnvelope struct {
	Plan SchedulePlan `json:"plan" yaml:"plan"`
}

type SchedulePlan struct {
	ID                string           `json:"id" yaml:"id"`
	WorkflowVersionID string           `json:"workflowVersionId" yaml:"workflowVersionId"`
	ExecutionScopeID  string           `json:"executionScopeId" yaml:"executionScopeId"`
	NetworkTopologyID string           `json:"networkTopologyId,omitempty" yaml:"networkTopologyId,omitempty"`
	Source            string           `json:"source" yaml:"source"`
	Algorithm         string           `json:"algorithm" yaml:"algorithm"`
	AlgorithmVersion  string           `json:"algorithmVersion" yaml:"algorithmVersion"`
	Objective         string           `json:"objective" yaml:"objective"`
	DeadlineSeconds   float64          `json:"deadlineSeconds" yaml:"deadlineSeconds"`
	Budget            float64          `json:"budget" yaml:"budget"`
	Predicted         PredictedMetrics `json:"predicted" yaml:"predicted"`
	Assignments       []Assignment     `json:"assignments" yaml:"assignments"`
	Metadata          map[string]any   `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type PredictedMetrics struct {
	MakespanSeconds float64 `json:"makespanSeconds" yaml:"makespanSeconds"`
	Cost            float64 `json:"cost" yaml:"cost"`
	Feasible        bool    `json:"feasible" yaml:"feasible"`
}

type Assignment struct {
	ID                       string         `json:"id" yaml:"id"`
	PlanID                   string         `json:"planId" yaml:"planId"`
	ActivityID               string         `json:"activityId" yaml:"activityId"`
	ResourceID               string         `json:"resourceId" yaml:"resourceId"`
	CoreID                   string         `json:"coreId,omitempty" yaml:"coreId,omitempty"`
	SlotID                   string         `json:"slotId,omitempty" yaml:"slotId,omitempty"`
	OrderOnResource          int            `json:"orderOnResource" yaml:"orderOnResource"`
	Priority                 int            `json:"priority" yaml:"priority"`
	PredictedReadyAt         float64        `json:"predictedReadyAt" yaml:"predictedReadyAt"`
	PredictedStartAt         float64        `json:"predictedStartAt" yaml:"predictedStartAt"`
	PredictedFinishAt        float64        `json:"predictedFinishAt" yaml:"predictedFinishAt"`
	PredictedRuntimeSeconds  float64        `json:"predictedRuntimeSeconds" yaml:"predictedRuntimeSeconds"`
	PredictedTransferSeconds float64        `json:"predictedTransferSeconds" yaml:"predictedTransferSeconds"`
	PredictedCost            float64        `json:"predictedCost" yaml:"predictedCost"`
	Metadata                 map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}
