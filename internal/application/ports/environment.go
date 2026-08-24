package ports

import (
	"context"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type ConnectionHealth struct {
	Healthy bool
	Message string
}

type ConnectionProber interface {
	Probe(context.Context, domain.EnvironmentConnection) ConnectionHealth
}

type ConnectionHealthMonitor interface {
	Check(context.Context, string) (domain.ConnectionCheck, error)
	History(context.Context, string, int) ([]domain.ConnectionCheck, error)
}

type DiscoveryRequest struct {
	Environment domain.Environment
	Version     domain.EnvironmentVersion
	Connections []domain.EnvironmentConnection
	Runtimes    []domain.EnvironmentRuntime
}

type DiscoveryResult struct {
	Resources       []domain.Resource
	Snapshots       []domain.ResourceSnapshot
	NetworkTopology domain.NetworkTopology
	Capabilities    domain.EnvironmentCapabilities
}

type EnvironmentConnector interface {
	Connect(context.Context, domain.EnvironmentConnection) error
	Health(context.Context, domain.EnvironmentConnection) (ConnectionHealth, error)
	Close(context.Context, domain.EnvironmentConnection) error
}

type EnvironmentDiscoverer interface {
	Discover(context.Context, DiscoveryRequest) (DiscoveryResult, error)
}

// ConnectionDiscovery is a bounded, read-only observation of one real
// infrastructure connection. Resource snapshots receive this data; plans do
// not trigger discovery themselves.
type ConnectionDiscovery struct {
	Available bool
	Metadata  map[string]any
	Warnings  []string
	Nodes     []DiscoveredNode
	LoginNode *DiscoveredLoginNode
	Transfer  domain.TransferCapabilities
}

// ConnectorBindingHealthChecker executes an explicit, credential-scoped
// health check. Discovery must never iterate bindings automatically.
type ConnectorBindingHealthChecker interface {
	CheckConnectorBinding(context.Context, domain.ConnectorBinding) (domain.ConnectorHealth, error)
}

type DiscoveredLoginNode struct {
	Name         string
	Architecture string
	CPUCores     int
	MemoryBytes  int64
	StorageBytes int64
	Metadata     map[string]any
}

// DiscoveredNode is the scheduler's current view of one physical compute node.
// Partitions contains provider IDs, not Akoflow resource IDs.
type DiscoveredNode struct {
	Name             string
	State            string
	CPUCores         int
	MemoryBytes      int64
	StorageBytes     int64
	Architecture     string
	Partitions       []string
	Features         []string
	GenericResources []string
	Reason           string
	Metadata         map[string]any
}

type ConnectionDiscoverer interface {
	DiscoverConnection(context.Context, domain.EnvironmentConnection) (ConnectionDiscovery, error)
}

type EnvironmentDiscovery interface {
	DiscoverConnection(context.Context, string) ([]domain.ResourceSnapshot, error)
}

type EnvironmentStore interface {
	UpdateStatus(context.Context, string, domain.EnvironmentStatus) error
	UpsertConnection(context.Context, domain.EnvironmentConnection) error
	ListConnections(context.Context, string) ([]domain.EnvironmentConnection, error)
}

type ConnectionStore interface {
	FindConnection(context.Context, string) (*domain.EnvironmentConnection, error)
	ListAllConnections(context.Context) ([]domain.EnvironmentConnection, error)
	SaveConnectionCheck(context.Context, domain.ConnectionCheck) error
	ListConnectionChecks(context.Context, string, int) ([]domain.ConnectionCheck, error)
}

type NetworkTopologyStore interface {
	Create(context.Context, domain.NetworkTopology) error
	List(context.Context) ([]domain.NetworkTopology, error)
	Find(context.Context, string) (*domain.NetworkTopology, error)
}

type ExecutionScopeStore interface {
	CreateScope(context.Context, domain.ExecutionScope) error
	DeleteScope(context.Context, string) error
	ListScopes(context.Context) ([]domain.ExecutionScope, error)
	FindScope(context.Context, string) (*domain.ExecutionScope, error)
}
