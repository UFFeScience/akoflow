package environment

import (
	"time"

	"github.com/UFFeScience/akoflow/internal/domain/resource"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
)

type EnvironmentVersionStatus string
type EnvironmentStatus string
type ConnectionType string
type DiscoveryRunStatus string
type RuntimeDriver string
type RuntimeMode string
type ConnectionStatus string

const (
	EnvironmentVersionDraft     EnvironmentVersionStatus = "draft"
	EnvironmentVersionPublished EnvironmentVersionStatus = "published"
	EnvironmentVersionRetired   EnvironmentVersionStatus = "retired"
)

const (
	ConnectionOnline  ConnectionStatus = "online"
	ConnectionOffline ConnectionStatus = "offline"
)

const (
	RuntimeDriverSlurm      RuntimeDriver = "slurm"
	RuntimeDriverKubernetes RuntimeDriver = "kubernetes"
	RuntimeDriverSSH        RuntimeDriver = "ssh"
	RuntimeDriverLocal      RuntimeDriver = "local"
	RuntimeDriverServerless RuntimeDriver = "serverless"
	RuntimeDriverSimGrid    RuntimeDriver = "simgrid"

	RuntimeModeExecution  RuntimeMode = "execution"
	RuntimeModeSimulation RuntimeMode = "simulation"
)

const (
	EnvironmentDefined     EnvironmentStatus = "defined"
	EnvironmentConnecting  EnvironmentStatus = "connecting"
	EnvironmentConnected   EnvironmentStatus = "connected"
	EnvironmentDiscovering EnvironmentStatus = "discovering"
	EnvironmentReady       EnvironmentStatus = "ready"
	EnvironmentDegraded    EnvironmentStatus = "degraded"
	EnvironmentUnreachable EnvironmentStatus = "unreachable"

	ConnectionSSH        ConnectionType = "ssh"
	ConnectionKubernetes ConnectionType = "kubernetes"
	ConnectionCloud      ConnectionType = "cloud"
	ConnectionLocal      ConnectionType = "local"
	ConnectionAgent      ConnectionType = "agent"
)

type Environment struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Status      EnvironmentStatus `json:"status"`
	CreatedAt   time.Time         `json:"createdAt"`
}

type EnvironmentConnection struct {
	ID            string         `json:"id"`
	EnvironmentID string         `json:"environmentId"`
	Name          string         `json:"name"`
	Type          ConnectionType `json:"type"`
	Endpoint      string         `json:"endpoint,omitempty"`
	Username      string         `json:"username,omitempty"`
	CredentialRef string         `json:"credentialRef,omitempty"`
	Configuration map[string]any `json:"configuration,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
}

type ConnectionCheck struct {
	ID           string           `json:"id"`
	ConnectionID string           `json:"connectionId"`
	Status       ConnectionStatus `json:"status"`
	Message      string           `json:"message,omitempty"`
	LatencyMS    float64          `json:"latencyMs"`
	CheckedAt    time.Time        `json:"checkedAt"`
	Metadata     map[string]any   `json:"metadata,omitempty"`
}

type Capabilities struct {
	Batch         bool `json:"batch"`
	Interactive   bool `json:"interactive"`
	Container     bool `json:"container"`
	Serverless    bool `json:"serverless"`
	GPU           bool `json:"gpu"`
	MPI           bool `json:"mpi"`
	SharedStorage bool `json:"sharedStorage"`
	DataStaging   bool `json:"dataStaging"`
	Cancellation  bool `json:"cancellation"`
	LogStreaming  bool `json:"logStreaming"`
	Simulation    bool `json:"simulation"`
}

type DiscoveryRun struct {
	ID                   string             `json:"id"`
	EnvironmentVersionID string             `json:"environmentVersionId"`
	Provider             string             `json:"provider"`
	Status               DiscoveryRunStatus `json:"status"`
	ResourcesFound       int                `json:"resourcesFound"`
	Error                string             `json:"error,omitempty"`
	StartedAt            time.Time          `json:"startedAt"`
	FinishedAt           *time.Time         `json:"finishedAt,omitempty"`
}

type EnvironmentVersion struct {
	ID                string                   `json:"id"`
	EnvironmentID     string                   `json:"environmentId"`
	Version           int                      `json:"version"`
	Status            EnvironmentVersionStatus `json:"status"`
	NetworkModel      string                   `json:"networkModel"`
	InterferenceModel string                   `json:"interferenceModel"`
	CostModel         string                   `json:"costModel"`
	ConfigurationHash string                   `json:"configurationHash"`
	CreatedAt         time.Time                `json:"createdAt"`
	PublishedAt       *time.Time               `json:"publishedAt,omitempty"`
}

type EnvironmentRuntime struct {
	EnvironmentVersionID string         `json:"environmentVersionId"`
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	Driver               RuntimeDriver  `json:"driver"`
	Mode                 RuntimeMode    `json:"mode"`
	Role                 string         `json:"role,omitempty"`
	Configuration        map[string]any `json:"configuration,omitempty"`
	Capabilities         Capabilities   `json:"capabilities"`
}

type ExecutionScope struct {
	ID                    string         `json:"id"`
	Name                  string         `json:"name"`
	NetworkTopologyID     string         `json:"networkTopologyId"`
	EnvironmentVersionIDs []string       `json:"environmentVersionIds"`
	Metadata              map[string]any `json:"metadata,omitempty"`
}

type Definition struct {
	Environment       Environment                        `json:"environment"`
	Version           EnvironmentVersion                 `json:"version"`
	Runtimes          []EnvironmentRuntime               `json:"runtimes"`
	Resources         []resource.Resource                `json:"resources"`
	RuntimeBindings   []resource.ResourceRuntimeBinding  `json:"resourceRuntimeBindings,omitempty"`
	Relations         []resource.ResourceRelation        `json:"resourceRelations,omitempty"`
	Storages          []resource.StorageResource         `json:"storages,omitempty"`
	Profiles          []workflow.ActivityResourceProfile `json:"activityResourceProfiles,omitempty"`
	Connections       []EnvironmentConnection            `json:"connections,omitempty"`
	ConnectionChecks  []ConnectionCheck                  `json:"connectionChecks,omitempty"`
	ConnectorBindings []ConnectorBinding                 `json:"connectorBindings,omitempty"`
}
