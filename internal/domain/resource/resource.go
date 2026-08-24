package resource

import "time"

type ResourceType string
type ResourceExecutionTarget string
type ResourceRelationType string

const (
	ResourceCluster             ResourceType = "cluster"
	ResourceNodePool            ResourceType = "node_pool"
	ResourceKubernetesMachine   ResourceType = "kubernetes_machine"
	ResourceHPCPartition        ResourceType = "hpc_partition"
	ResourceHPCMachine          ResourceType = "hpc_machine"
	ResourceCloudVM             ResourceType = "cloud_vm"
	ResourceFogDevice           ResourceType = "fog_device"
	ResourceLocalMachine        ResourceType = "local_machine"
	ResourceServerlessPlatform  ResourceType = "serverless_platform"
	ResourceServerlessFunction  ResourceType = "serverless_function"
	ResourceBatchQueue          ResourceType = "batch_queue"
	ResourceKubernetesNamespace ResourceType = "kubernetes_namespace"
	ResourceSlurmReservation    ResourceType = "slurm_reservation"

	ExecutionTargetBatch  ResourceExecutionTarget = "batch"
	ExecutionTargetDirect ResourceExecutionTarget = "direct"
)

const (
	ResourceRelationContains      ResourceRelationType = "contains"
	ResourceRelationMemberOf      ResourceRelationType = "member_of"
	ResourceRelationAccessibleVia ResourceRelationType = "accessible_via"
)

type Resource struct {
	ID                   string                  `json:"id"`
	EnvironmentVersionID string                  `json:"environmentVersionId"`
	ExecutionTarget      ResourceExecutionTarget `json:"executionTarget,omitempty"`
	ParentResourceID     *string                 `json:"parentResourceId,omitempty"`
	Type                 ResourceType            `json:"type"`
	Name                 string                  `json:"name"`
	ProviderID           string                  `json:"providerId"`
	Tier                 string                  `json:"tier,omitempty"`
	Region               string                  `json:"region,omitempty"`
	Zone                 string                  `json:"zone,omitempty"`
	Architecture         string                  `json:"architecture,omitempty"`
	CPUCores             int                     `json:"cpuCores"`
	CPUCapacity          float64                 `json:"cpuCapacity"`
	MemoryBytes          int64                   `json:"memoryBytes"`
	StorageBytes         int64                   `json:"storageBytes"`
	ComputeSpeedup       float64                 `json:"computeSpeedup"`
	PricePerSecond       float64                 `json:"pricePerSecond"`
	BootOverheadSeconds  float64                 `json:"bootOverheadSeconds"`
	ContainerOverhead    float64                 `json:"containerOverheadSeconds"`
	Schedulable          bool                    `json:"schedulable"`
	Metadata             map[string]any          `json:"metadata,omitempty"`
}

type ResourceRuntimeBinding struct {
	ResourceID    string         `json:"resourceId"`
	RuntimeID     string         `json:"runtimeId"`
	Enabled       bool           `json:"enabled"`
	Configuration map[string]any `json:"configuration,omitempty"`
}

type ResourceRelation struct {
	EnvironmentVersionID string               `json:"environmentVersionId"`
	SourceResourceID     string               `json:"sourceResourceId"`
	TargetResourceID     string               `json:"targetResourceId"`
	Type                 ResourceRelationType `json:"type"`
	Metadata             map[string]any       `json:"metadata,omitempty"`
}

type ResourceSnapshot struct {
	ID              string         `json:"id"`
	ResourceID      string         `json:"resourceId"`
	CapturedAt      time.Time      `json:"capturedAt"`
	Available       bool           `json:"available"`
	CPUUsed         float64        `json:"cpuUsed"`
	MemoryUsedBytes int64          `json:"memoryUsedBytes"`
	NetworkInBPS    float64        `json:"networkInBps"`
	NetworkOutBPS   float64        `json:"networkOutBps"`
	DiskReadBPS     float64        `json:"diskReadBps"`
	DiskWriteBPS    float64        `json:"diskWriteBps"`
	QueueLength     int            `json:"queueLength"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type NetworkLink struct {
	ID                     string         `json:"id"`
	TopologyID             string         `json:"topologyId"`
	SourceResourceID       string         `json:"sourceResourceId"`
	TargetResourceID       string         `json:"targetResourceId"`
	BandwidthBitsPerSecond float64        `json:"bandwidthBitsPerSecond"`
	LatencySeconds         float64        `json:"latencySeconds"`
	PricePerByte           float64        `json:"pricePerByte"`
	Bidirectional          bool           `json:"bidirectional"`
	SharingPolicy          string         `json:"sharingPolicy,omitempty"`
	MaxConcurrentTransfers int            `json:"maxConcurrentTransfers,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`
}

type NetworkTopology struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Version          int            `json:"version"`
	ExecutionScopeID string         `json:"executionScopeId"`
	Links            []NetworkLink  `json:"links"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

func (l NetworkLink) TransferSeconds(bytes int64) float64 {
	if bytes <= 0 {
		return l.LatencySeconds
	}
	if l.BandwidthBitsPerSecond <= 0 {
		return 0
	}
	return l.LatencySeconds + float64(bytes)/(l.BandwidthBitsPerSecond/8.0)
}
