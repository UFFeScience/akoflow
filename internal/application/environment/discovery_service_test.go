package environment

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type discoveryCatalog struct {
	definition domain.EnvironmentDefinition
	storages   []domain.StorageResource
}

func (c discoveryCatalog) Create(context.Context, domain.EnvironmentDefinition) error  { return nil }
func (c discoveryCatalog) Replace(context.Context, domain.EnvironmentDefinition) error { return nil }
func (c discoveryCatalog) Delete(context.Context, string) error                        { return nil }
func (c discoveryCatalog) List(context.Context) ([]domain.EnvironmentDefinition, error) {
	return []domain.EnvironmentDefinition{c.definition}, nil
}
func (c discoveryCatalog) Find(context.Context, string) (*domain.EnvironmentDefinition, error) {
	return &c.definition, nil
}
func (discoveryCatalog) UpdateStatus(context.Context, string, domain.EnvironmentStatus) error {
	return nil
}
func (discoveryCatalog) UpsertConnection(context.Context, domain.EnvironmentConnection) error {
	return nil
}
func (discoveryCatalog) ListConnections(context.Context, string) ([]domain.EnvironmentConnection, error) {
	return nil, nil
}
func (c *discoveryCatalog) UpsertDiscoveredStorage(_ context.Context, value domain.StorageResource) error {
	c.storages = append(c.storages, value)
	return nil
}

type discoveryInventory struct {
	resources []domain.Resource
	bindings  []domain.ResourceRuntimeBinding
	relations []domain.ResourceRelation
	snapshots []domain.ResourceSnapshot
}

func (i *discoveryInventory) Upsert(_ context.Context, value domain.Resource) error {
	i.resources = append(i.resources, value)
	return nil
}
func (i *discoveryInventory) UpsertRuntimeBinding(_ context.Context, value domain.ResourceRuntimeBinding) error {
	i.bindings = append(i.bindings, value)
	return nil
}
func (i *discoveryInventory) UpsertRelation(_ context.Context, value domain.ResourceRelation) error {
	i.relations = append(i.relations, value)
	return nil
}
func (*discoveryInventory) List(context.Context) ([]domain.Resource, error) { return nil, nil }
func (*discoveryInventory) FindByID(context.Context, string) (*domain.Resource, error) {
	return nil, nil
}
func (*discoveryInventory) FindByProviderID(context.Context, string, string) (*domain.Resource, error) {
	return nil, nil
}
func (*discoveryInventory) ListByRuntime(context.Context, string, string) ([]domain.Resource, error) {
	return nil, nil
}
func (*discoveryInventory) ListSchedulable(context.Context, string) ([]domain.Resource, error) {
	return nil, nil
}
func (i *discoveryInventory) CreateSnapshot(_ context.Context, value domain.ResourceSnapshot) error {
	i.snapshots = append(i.snapshots, value)
	return nil
}
func (*discoveryInventory) LatestSnapshot(context.Context, string) (*domain.ResourceSnapshot, error) {
	return nil, nil
}

type connectionDiscovererStub struct{ observation ports.ConnectionDiscovery }

func (d connectionDiscovererStub) DiscoverConnection(context.Context, domain.EnvironmentConnection) (ports.ConnectionDiscovery, error) {
	return d.observation, nil
}

func TestDiscoveryMaterializesSlurmNodesAndPartitionRelations(t *testing.T) {
	clusterID := "cluster"
	definition := domain.EnvironmentDefinition{
		Version:     domain.EnvironmentVersion{ID: "env-v1"},
		Connections: []domain.EnvironmentConnection{{ID: "plafrim", Type: domain.ConnectionSSH}},
		Runtimes:    []domain.EnvironmentRuntime{{ID: "slurm", Configuration: map[string]any{"connectionId": "plafrim"}}},
		Resources: []domain.Resource{
			{ID: clusterID, Type: domain.ResourceCluster},
			{ID: "routage", ParentResourceID: &clusterID, Type: domain.ResourceHPCPartition, Name: "routage", ProviderID: "routage"},
		},
		RuntimeBindings: []domain.ResourceRuntimeBinding{{ResourceID: "routage", RuntimeID: "slurm", Enabled: true}},
	}
	inventory := &discoveryInventory{}
	coordinator := NewDiscoveryCoordinator(discoveryCatalog{definition: definition}, inventory,
		map[domain.ConnectionType]ports.ConnectionDiscoverer{domain.ConnectionSSH: connectionDiscovererStub{observation: ports.ConnectionDiscovery{
			Available: true, Metadata: map[string]any{"source": "test"}, LoginNode: &ports.DiscoveredLoginNode{
				Name: "login.plafrim.fr", CPUCores: 8, MemoryBytes: 16 * 1024 * 1024 * 1024,
			}, Nodes: []ports.DiscoveredNode{{
				Name: "bora001", State: "idle", CPUCores: 48, MemoryBytes: 1024, Partitions: []string{"routage"},
			}},
		}}})
	snapshots, err := coordinator.DiscoverConnection(context.Background(), "plafrim")
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.resources) != 4 {
		t.Fatalf("resources=%+v", inventory.resources)
	}
	var login, node *domain.Resource
	for index := range inventory.resources {
		resource := &inventory.resources[index]
		if resource.Metadata["role"] == "login" {
			login = resource
		}
		if resource.ProviderID == "bora001" {
			node = resource
		}
	}
	if login == nil || login.ExecutionTarget != domain.ExecutionTargetDirect || login.ParentResourceID == nil {
		t.Fatalf("login=%+v", login)
	}
	if login.Schedulable {
		t.Fatalf("login node must remain a gateway, not an activity scheduling target: %+v", login)
	}
	if node == nil || node.Type != domain.ResourceHPCMachine {
		t.Fatalf("node=%+v", node)
	}
	if len(inventory.bindings) != 4 {
		t.Fatalf("bindings=%+v", inventory.bindings)
	}
	if len(inventory.relations) != 1 || inventory.relations[0].SourceResourceID != "routage" {
		t.Fatalf("relations=%+v", inventory.relations)
	}
	if len(snapshots) != 3 || !snapshots[0].Available {
		t.Fatalf("snapshots=%+v", snapshots)
	}
}

func TestDiscoveryMaterializesPartitionsReportedBySlurm(t *testing.T) {
	definition := domain.EnvironmentDefinition{Version: domain.EnvironmentVersion{ID: "env-v1"}, Connections: []domain.EnvironmentConnection{{ID: "hpc", Type: domain.ConnectionSSH}}, Runtimes: []domain.EnvironmentRuntime{{ID: "slurm", Configuration: map[string]any{"connectionId": "hpc"}}}, Resources: []domain.Resource{{ID: "cluster", Type: domain.ResourceCluster, ProviderID: "cluster"}}, RuntimeBindings: []domain.ResourceRuntimeBinding{{ResourceID: "cluster", RuntimeID: "slurm", Enabled: true}}}
	inventory := &discoveryInventory{}
	coordinator := NewDiscoveryCoordinator(discoveryCatalog{definition: definition}, inventory, map[domain.ConnectionType]ports.ConnectionDiscoverer{domain.ConnectionSSH: connectionDiscovererStub{observation: ports.ConnectionDiscovery{Available: true, LoginNode: &ports.DiscoveredLoginNode{Name: "login"}, Metadata: map[string]any{"partitions": []map[string]any{{"name": "cpu", "cpuCoresPerNode": int64(64), "memoryMiBPerNode": int64(256)}}}}}})
	if _, err := coordinator.DiscoverConnection(context.Background(), "hpc"); err != nil {
		t.Fatal(err)
	}
	var partition *domain.Resource
	for index := range inventory.resources {
		if inventory.resources[index].Type == domain.ResourceHPCPartition {
			partition = &inventory.resources[index]
			break
		}
	}
	if partition == nil || partition.ProviderID != "cpu" || !partition.Schedulable {
		t.Fatalf("partition=%+v", partition)
	}
	if len(inventory.bindings) == 0 {
		t.Fatal("discovered partition must be bound to the SLURM runtime")
	}
}

func TestDiscoveryBindsDiscoveredStorageToConnectionRuntimes(t *testing.T) {
	definition := domain.EnvironmentDefinition{
		Version:         domain.EnvironmentVersion{ID: "env-v1"},
		Connections:     []domain.EnvironmentConnection{{ID: "hpc", Type: domain.ConnectionSSH}},
		Runtimes:        []domain.EnvironmentRuntime{{ID: "slurm", Configuration: map[string]any{"connectionId": "hpc"}}},
		Resources:       []domain.Resource{{ID: "cluster", Type: domain.ResourceCluster}},
		RuntimeBindings: []domain.ResourceRuntimeBinding{{ResourceID: "cluster", RuntimeID: "slurm", Enabled: true}},
	}
	catalog := &discoveryCatalog{definition: definition}
	coordinator := NewDiscoveryCoordinator(catalog, &discoveryInventory{}, map[domain.ConnectionType]ports.ConnectionDiscoverer{
		domain.ConnectionSSH: connectionDiscovererStub{observation: ports.ConnectionDiscovery{
			Available: true, Transfer: domain.TransferCapabilities{Paths: []domain.TransferPath{{Path: "/scratch/user", Kind: "scratch", Writable: true}}},
		}},
	})
	if _, err := coordinator.DiscoverConnection(context.Background(), "hpc"); err != nil {
		t.Fatal(err)
	}
	if len(catalog.storages) != 1 || len(catalog.storages[0].RuntimeBindings) != 1 {
		t.Fatalf("storages=%+v", catalog.storages)
	}
	binding := catalog.storages[0].RuntimeBindings[0]
	if binding.RuntimeID != "slurm" || binding.HostPath != "/scratch/user" {
		t.Fatalf("binding=%+v", binding)
	}
}

func TestDiscoveryMaterializesDirectSSHMachine(t *testing.T) {
	definition := domain.EnvironmentDefinition{
		Environment: domain.Environment{ID: "remote"}, Version: domain.EnvironmentVersion{ID: "remote-v1"},
		Connections:     []domain.EnvironmentConnection{{ID: "ssh", EnvironmentID: "remote", Type: domain.ConnectionSSH, Configuration: map[string]any{"skipSchedulerCheck": true}}},
		Runtimes:        []domain.EnvironmentRuntime{{ID: "docker", Configuration: map[string]any{"connectionId": "ssh"}}},
		Resources:       []domain.Resource{{ID: "machine", Type: domain.ResourceHPCMachine, Metadata: map[string]any{"configured": true}}},
		RuntimeBindings: []domain.ResourceRuntimeBinding{{ResourceID: "machine", RuntimeID: "docker", Enabled: true}},
	}
	inventory := &discoveryInventory{}
	coordinator := NewDiscoveryCoordinator(discoveryCatalog{definition: definition}, inventory,
		map[domain.ConnectionType]ports.ConnectionDiscoverer{domain.ConnectionSSH: connectionDiscovererStub{observation: ports.ConnectionDiscovery{
			Available: true, Metadata: map[string]any{
				"architecture": "amd64", "cpuCores": float64(8), "memoryBytes": int64(16384),
				"storageBytes": 32768, "dockerAvailable": true, "hostname": "remote.example",
			},
		}}})
	snapshots, err := coordinator.DiscoverConnection(context.Background(), "ssh")
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.resources) != 1 {
		t.Fatalf("resources=%+v", inventory.resources)
	}
	resource := inventory.resources[0]
	if resource.Type != domain.ResourceCloudVM || resource.ExecutionTarget != domain.ExecutionTargetDirect ||
		resource.CPUCores != 8 || resource.MemoryBytes != 16384 || resource.StorageBytes != 32768 ||
		resource.Metadata["observedHostname"] != "remote.example" || !resource.Schedulable {
		t.Fatalf("resource=%+v", resource)
	}
	if len(snapshots) != 1 || snapshots[0].ResourceID != "machine" {
		t.Fatalf("snapshots=%+v", snapshots)
	}
}

func TestNumericDiscoveryHelpers(t *testing.T) {
	if int64Value(int64(1)) != 1 || int64Value(2) != 2 || int64Value(float64(3)) != 3 || int64Value("4") != 0 {
		t.Fatal("unexpected int64 conversion")
	}
	if intValue(int64(1)) != 1 || intValue(2) != 2 || intValue(float64(3)) != 3 || intValue("4") != 0 {
		t.Fatal("unexpected int conversion")
	}
}
