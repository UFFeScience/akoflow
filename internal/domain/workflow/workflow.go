package workflow

import (
	"fmt"
)

type ActivityKind string
type ActivityCapability string

const (
	ActivityKindTask        ActivityKind = "task"
	ActivityKindService     ActivityKind = "service"
	ActivityKindInteractive ActivityKind = "interactive"

	ActivityCapabilityReal        ActivityCapability = "real"
	ActivityCapabilitySimulation  ActivityCapability = "simulation"
	ActivityCapabilityInteractive ActivityCapability = "interactive"
)

type ActivityCommand struct {
	// Image is the legacy shorthand for an OCI reference or a destination path.
	// New definitions should use Executable. It is kept so existing workflow
	// definitions remain valid while the planner resolves them.
	Image              string               `json:"image,omitempty"`
	Executable         *ExecutableReference `json:"executable,omitempty"`
	ResolvedExecutable *ResolvedExecutable  `json:"resolvedExecutable,omitempty"`
	Entrypoint         string               `json:"entrypoint"`
	Arguments          []string             `json:"arguments,omitempty"`
	Environment        map[string]string    `json:"environment,omitempty"`
	WorkingDirectory   string               `json:"workingDirectory,omitempty"`
}

type ActivityResources struct {
	CPU          float64 `json:"cpu"`
	MemoryBytes  int64   `json:"memoryBytes"`
	StorageBytes int64   `json:"storageBytes"`
	GPU          int     `json:"gpu,omitempty"`
}

type ServiceSpec struct {
	Ports                 []int   `json:"ports,omitempty"`
	HealthCheck           string  `json:"healthCheck,omitempty"`
	StartupTimeoutSeconds float64 `json:"startupTimeoutSeconds,omitempty"`
	IdleTimeoutSeconds    float64 `json:"idleTimeoutSeconds,omitempty"`
	KeepAlive             bool    `json:"keepAlive,omitempty"`
}

type ActivitySimulation struct {
	Model           string         `json:"model,omitempty"`
	DurationSeconds float64        `json:"durationSeconds,omitempty"`
	FLOPs           float64        `json:"flops,omitempty"`
	Parameters      map[string]any `json:"parameters,omitempty"`
}

type ActivityPolicy struct {
	TimeoutSeconds    float64 `json:"timeoutSeconds,omitempty"`
	MaxAttempts       int     `json:"maxAttempts,omitempty"`
	RetryDelaySeconds float64 `json:"retryDelaySeconds,omitempty"`
}

type WorkflowVersion struct {
	ID               string                   `json:"id"`
	WorkflowID       string                   `json:"workflowId"`
	Version          int                      `json:"version"`
	DefinitionHash   string                   `json:"definitionHash"`
	Activities       []Activity               `json:"activities"`
	Dependencies     []ActivityDependency     `json:"dependencies"`
	DataDependencies []ActivityDataDependency `json:"dataDependencies,omitempty"`
}

type Definition struct {
	ID         string          `json:"id"`
	ExternalID string          `json:"externalId"`
	Name       string          `json:"name"`
	Namespace  string          `json:"namespace"`
	Version    WorkflowVersion `json:"version"`
	Types      []ActivityType  `json:"activityTypes"`
}

type ActivityType struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Application      string         `json:"application,omitempty"`
	DefaultImage     string         `json:"defaultImage,omitempty"`
	CPUIntensity     float64        `json:"cpuIntensity"`
	MemoryIntensity  float64        `json:"memoryIntensity"`
	IOIntensity      float64        `json:"ioIntensity"`
	NetworkIntensity float64        `json:"networkIntensity"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type Activity struct {
	ID                string               `json:"id"`
	WorkflowVersionID string               `json:"workflowVersionId"`
	ActivityTypeID    string               `json:"activityTypeId"`
	ExternalID        string               `json:"externalId"`
	Name              string               `json:"name"`
	Kind              ActivityKind         `json:"kind"`
	Capabilities      []ActivityCapability `json:"capabilities"`
	Command           ActivityCommand      `json:"command"`
	Resources         ActivityResources    `json:"resources"`
	Service           *ServiceSpec         `json:"service,omitempty"`
	Simulation        *ActivitySimulation  `json:"simulation,omitempty"`
	Policy            ActivityPolicy       `json:"policy"`
	Priority          int                  `json:"priority"`
	Metadata          map[string]any       `json:"metadata,omitempty"`
}

func (a Activity) Supports(capability ActivityCapability) bool {
	for _, supported := range a.Capabilities {
		if supported == capability {
			return true
		}
	}
	return false
}

func (a Activity) Validate() error {
	if a.ID == "" || a.Name == "" {
		return fmt.Errorf("activity id and name are required")
	}
	if a.Kind == "" {
		return fmt.Errorf("activity %q kind is required", a.ID)
	}
	if len(a.Capabilities) == 0 {
		return fmt.Errorf("activity %q must declare at least one execution capability", a.ID)
	}
	if a.Supports(ActivityCapabilityReal) && a.Command.Entrypoint == "" {
		return fmt.Errorf("activity %q requires an entrypoint for real execution", a.ID)
	}
	if a.Supports(ActivityCapabilitySimulation) && a.Simulation == nil {
		return fmt.Errorf("activity %q requires a simulation definition", a.ID)
	}
	if (a.Kind == ActivityKindService || a.Kind == ActivityKindInteractive) && a.Service == nil {
		return fmt.Errorf("activity %q requires a service definition", a.ID)
	}
	if a.Supports(ActivityCapabilityInteractive) && a.Kind != ActivityKindInteractive && a.Kind != ActivityKindService {
		return fmt.Errorf("activity %q must be a service or interactive to support interactive execution", a.ID)
	}
	return nil
}

type ActivityDependency struct {
	ActivityID          string `json:"activityId"`
	DependsOnActivityID string `json:"dependsOnActivityId"`
	Type                string `json:"type,omitempty"`
}

type ActivityDataDependency struct {
	ProducerActivityID string `json:"producerActivityId"`
	ConsumerActivityID string `json:"consumerActivityId"`
	LogicalName        string `json:"logicalName"`
	SizeBytes          int64  `json:"sizeBytes"`
}

type ActivityResourceProfile struct {
	ID                   string         `json:"id"`
	ActivityTypeID       string         `json:"activityTypeId"`
	ResourceID           string         `json:"resourceId"`
	RuntimeSeconds       float64        `json:"runtimeSeconds"`
	RuntimeStdDevSeconds float64        `json:"runtimeStdDevSeconds"`
	CPUUtilization       float64        `json:"cpuUtilization"`
	PeakMemoryBytes      int64          `json:"peakMemoryBytes"`
	DiskReadBytes        int64          `json:"diskReadBytes"`
	DiskWriteBytes       int64          `json:"diskWriteBytes"`
	EnergyJoules         float64        `json:"energyJoules"`
	Source               string         `json:"source"`
	SampleSize           int            `json:"sampleSize"`
	ModelVersion         string         `json:"modelVersion"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}
