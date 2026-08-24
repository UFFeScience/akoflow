package environment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	"github.com/UFFeScience/akoflow/internal/provider"
)

// DiscoveryCoordinator executes one cheap, read-only probe per connector and
// persists the resulting effective capabilities on its bound resources.
type DiscoveryCoordinator struct {
	catalog     ports.EnvironmentCatalog
	resources   ports.ResourceInventory
	discoverers map[domain.ConnectionType]ports.ConnectionDiscoverer
	audit       ports.AuditStore
}

var _ ports.EnvironmentDiscovery = (*DiscoveryCoordinator)(nil)

func NewDiscoveryCoordinator(catalog ports.EnvironmentCatalog, resources ports.ResourceInventory, discoverers map[domain.ConnectionType]ports.ConnectionDiscoverer, audits ...ports.AuditStore) *DiscoveryCoordinator {
	coordinator := &DiscoveryCoordinator{catalog: catalog, resources: resources, discoverers: discoverers}
	if len(audits) > 0 {
		coordinator.audit = audits[0]
	}
	return coordinator
}

func (s *DiscoveryCoordinator) DiscoverConnection(ctx context.Context, connectionID string) ([]domain.ResourceSnapshot, error) {
	definition, connection, err := s.findConnection(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	discoverer := s.discoverers[connection.Type]
	if discoverer == nil {
		return nil, fmt.Errorf("no discovery driver is configured for connection type %q", connection.Type)
	}
	s.recordAudit(ctx, domainaudit.Event{EventType: "resource.discovery.started", EnvironmentID: definition.Environment.ID,
		ConnectionID: connection.ID, Outcome: domainaudit.OutcomeStarted, Summary: "Infrastructure discovery started"})
	observation, err := discoverer.DiscoverConnection(ctx, connection)
	if err != nil {
		s.recordAudit(ctx, domainaudit.Event{EventType: "resource.discovery.failed", EnvironmentID: definition.Environment.ID,
			ConnectionID: connection.ID, Outcome: domainaudit.OutcomeFailed, Summary: err.Error()})
		return nil, err
	}
	structureSnapshots, err := s.materializeHPCStructure(ctx, *definition, connection, observation)
	if err != nil {
		return nil, err
	}
	if stores, ok := s.catalog.(ports.DiscoveredStorageStore); ok {
		if err := materializeDiscoveredStorages(ctx, stores, *definition, connection, observation); err != nil {
			return nil, err
		}
	}
	resourceIDs := boundResourceIDs(*definition, connection.ID)
	if len(resourceIDs) == 0 {
		return nil, fmt.Errorf("connection %q has no bound resources", connection.ID)
	}
	now := time.Now().UTC()
	items := make([]domain.ResourceSnapshot, 0, len(resourceIDs)+len(structureSnapshots))
	items = append(items, structureSnapshots...)
	materialized := make(map[string]bool, len(structureSnapshots))
	for _, snapshot := range structureSnapshots {
		materialized[snapshot.ResourceID] = true
	}
	for _, resourceID := range resourceIDs {
		if materialized[resourceID] {
			continue
		}
		metadata := cloneMetadata(observation.Metadata)
		metadata["connectionId"] = connection.ID
		metadata["warnings"] = observation.Warnings
		metadata["discoverySource"] = string(connection.Type)
		snapshot := domain.ResourceSnapshot{
			ID: provider.NewID("resource-discovery"), ResourceID: resourceID,
			CapturedAt: now, Available: observation.Available, Metadata: metadata,
		}
		if err := s.resources.CreateSnapshot(ctx, snapshot); err != nil {
			return nil, err
		}
		items = append(items, snapshot)
	}
	s.recordAudit(ctx, domainaudit.Event{EventType: "resource.discovery.completed", EnvironmentID: definition.Environment.ID,
		ConnectionID: connection.ID, Outcome: domainaudit.OutcomeSucceeded, Summary: "Infrastructure discovery completed",
		Metadata: map[string]any{"nodeCount": len(observation.Nodes), "snapshotCount": len(items), "loginNodeDiscovered": observation.LoginNode != nil}})
	return items, nil
}

func (s *DiscoveryCoordinator) recordAudit(ctx context.Context, event domainaudit.Event) {
	if s.audit == nil {
		return
	}
	event.ID, event.OccurredAt = provider.NewID("audit"), time.Now().UTC()
	_ = s.audit.RecordAuditEvent(ctx, event)
}

// materializeDiscoveredStorages records only the bounded filesystem/transfer
// paths reported by the login-node probe. It never scans their contents.
func materializeDiscoveredStorages(ctx context.Context, store ports.DiscoveredStorageStore, definition domain.EnvironmentDefinition, connection domain.EnvironmentConnection, observation ports.ConnectionDiscovery) error {
	seen := map[string]bool{}
	runtimeIDs := runtimeIDsForConnection(definition, connection.ID)
	for _, p := range observation.Transfer.Paths {
		path := strings.TrimSpace(p.Path)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		kind := domain.StorageSSH
		if strings.Contains(strings.ToLower(p.Kind), "scratch") || strings.Contains(strings.ToLower(path), "scratch") {
			kind = domain.StorageLustre
		}
		id := "discovered-storage-" + connection.ID + "-" + strings.NewReplacer("/", "-", " ", "-").Replace(strings.Trim(path, "/"))
		if strings.HasSuffix(id, "-") {
			id += "root"
		}
		computeVisible := p.ComputeNodeVisible != nil && *p.ComputeNodeVisible
		value := domain.StorageResource{
			ID: id, EnvironmentVersionID: definition.Version.ID,
			Name: "Discovered " + p.Kind + " " + path, Type: kind,
			Endpoint: path, CapacityBytes: p.AvailableBytes,
			Shared: computeVisible, ReadOnly: !p.Writable,
			BrowseRoots: []domain.StorageBrowseRoot{{
				Path: path, Label: p.Kind, ReadOnly: !p.Writable,
			}},
			Capabilities: domain.StorageCapabilities{
				Browse: true, Read: true, Write: p.Writable, Download: true,
				Upload: p.Writable, Checksum: observation.Transfer.Checksum.Available,
				ComputeNodeVisible: computeVisible,
			},
			Configuration: map[string]any{"browseRoots": []any{map[string]any{
				"path": path, "label": p.Kind, "readOnly": !p.Writable,
			}}},
			Metadata: map[string]any{
				"connectionId": connection.ID, "discoverySource": "transfer-path",
			},
		}
		for _, runtimeID := range runtimeIDs {
			value.RuntimeBindings = append(value.RuntimeBindings, domain.StorageRuntimeBinding{
				RuntimeID: runtimeID, HostPath: path, ReadOnly: !p.Writable,
				Configuration: map[string]any{"discoverySource": "transfer-path"},
			})
		}
		if err := store.UpsertDiscoveredStorage(ctx, value); err != nil {
			return fmt.Errorf("upsert discovered storage %s: %w", path, err)
		}
	}
	return nil
}

func (s *DiscoveryCoordinator) materializeHPCStructure(ctx context.Context, definition domain.EnvironmentDefinition, connection domain.EnvironmentConnection, observation ports.ConnectionDiscovery) ([]domain.ResourceSnapshot, error) {
	if observation.LoginNode == nil && len(observation.Nodes) == 0 {
		return nil, nil
	}
	clusterID := clusterResourceID(connection.ID)
	for _, resource := range definition.Resources {
		if resource.Type == domain.ResourceCluster {
			clusterID = resource.ID
			break
		}
	}
	cluster := domain.Resource{ID: clusterID, EnvironmentVersionID: definition.Version.ID,
		Type: domain.ResourceCluster, Name: clusterResourceName(definition, connection), ProviderID: clusterID,
		ComputeSpeedup: 1, Schedulable: false, Metadata: map[string]any{"connectionId": connection.ID, "discovered": true}}
	if err := s.resources.Upsert(ctx, cluster); err != nil {
		return nil, fmt.Errorf("upsert discovered HPC cluster: %w", err)
	}
	runtimeIDs := runtimeIDsForConnection(definition, connection.ID)
	for _, runtimeID := range runtimeIDs {
		if err := s.resources.UpsertRuntimeBinding(ctx, domain.ResourceRuntimeBinding{ResourceID: clusterID, RuntimeID: runtimeID, Enabled: true}); err != nil {
			return nil, fmt.Errorf("bind discovered HPC cluster: %w", err)
		}
	}
	// SLURM partitions are discovered inventory, not static configuration. A
	// newly connected HPC environment therefore becomes schedulable without
	// requiring users to predeclare every queue in its YAML.
	definition.Resources = append(definition.Resources, discoveredPartitions(definition, connection, observation)...)
	for _, partition := range definition.Resources {
		if partition.Type != domain.ResourceHPCPartition {
			continue
		}
		partition.ParentResourceID = &clusterID
		if err := s.resources.Upsert(ctx, partition); err != nil {
			return nil, fmt.Errorf("attach partition %q to discovered cluster: %w", partition.ProviderID, err)
		}
		for _, runtimeID := range runtimeIDs {
			if err := s.resources.UpsertRuntimeBinding(ctx, domain.ResourceRuntimeBinding{ResourceID: partition.ID, RuntimeID: runtimeID, Enabled: true, Configuration: map[string]any{"executionTarget": "batch", "scheduler": "slurm"}}); err != nil {
				return nil, fmt.Errorf("bind discovered partition %q: %w", partition.ProviderID, err)
			}
		}
	}
	snapshots := []domain.ResourceSnapshot{}
	if observation.LoginNode != nil {
		snapshot, err := s.materializeLoginNode(ctx, definition, connection, clusterID, runtimeIDs, *observation.LoginNode, observation.Available)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	nodeSnapshots, err := s.materializeNodes(ctx, definition, connection, clusterID, observation)
	if err != nil {
		return nil, err
	}
	return append(snapshots, nodeSnapshots...), nil
}

func discoveredPartitions(definition domain.EnvironmentDefinition, connection domain.EnvironmentConnection, observation ports.ConnectionDiscovery) []domain.Resource {
	configured := map[string]bool{}
	for _, resource := range definition.Resources {
		if resource.Type == domain.ResourceHPCPartition {
			configured[resource.ProviderID] = true
		}
	}
	values, _ := observation.Metadata["partitions"].([]map[string]any)
	partitions := make([]domain.Resource, 0, len(values))
	for _, value := range values {
		name, _ := value["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" || configured[name] {
			continue
		}
		metadata := cloneMetadata(value)
		metadata["connectionId"], metadata["discovered"], metadata["interactive"] = connection.ID, true, true
		partitions = append(partitions, domain.Resource{
			ID: connection.ID + "-partition-" + resourceIdentifier(name), EnvironmentVersionID: definition.Version.ID,
			ExecutionTarget: domain.ExecutionTargetBatch, Type: domain.ResourceHPCPartition, Name: name, ProviderID: name,
			CPUCores: intValue(value["cpuCoresPerNode"]), CPUCapacity: float64(intValue(value["cpuCoresPerNode"])),
			MemoryBytes: int64(intValue(value["memoryMiBPerNode"])) * 1024 * 1024, ComputeSpeedup: 1, Schedulable: true, Metadata: metadata,
		})
	}
	return partitions
}

func resourceIdentifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' || character == '_' {
			builder.WriteRune(character)
		} else {
			builder.WriteByte('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}

func intValue(value any) int {
	switch value := value.(type) {
	case int64:
		return int(value)
	case int:
		return value
	case float64:
		return int(value)
	default:
		return 0
	}
}

func (s *DiscoveryCoordinator) materializeLoginNode(
	ctx context.Context,
	definition domain.EnvironmentDefinition,
	connection domain.EnvironmentConnection,
	clusterID string,
	runtimeIDs []string,
	discovered ports.DiscoveredLoginNode,
	available bool,
) (domain.ResourceSnapshot, error) {
	resourceID := loginResourceID(connection.ID)
	metadata := cloneMetadata(discovered.Metadata)
	metadata["connectionId"], metadata["discovered"] = connection.ID, true
	role, _ := metadata["role"].(string)
	if strings.TrimSpace(role) == "" {
		metadata["role"] = "login"
	}
	metadata["observedHostname"] = discovered.Name
	metadata["interactive"] = true
	metadata["commandExecution"] = true
	if metadata["role"] == "login" {
		metadata["schedulerGateway"] = true
	}
	metadata["maxDirectCpu"] = minPositive(discovered.CPUCores, 2)
	metadata["maxDirectMemoryBytes"] = minPositive64(discovered.MemoryBytes, 4*1024*1024*1024)
	metadata["maxDirectDurationSeconds"] = 1800
	resource := domain.Resource{ID: resourceID, EnvironmentVersionID: definition.Version.ID, ParentResourceID: &clusterID,
		ExecutionTarget: domain.ExecutionTargetDirect, Type: domain.ResourceHPCMachine, Name: connection.Name,
		ProviderID: connection.Endpoint, Architecture: discovered.Architecture, CPUCores: discovered.CPUCores,
		CPUCapacity: float64(minPositive(discovered.CPUCores, 2)), MemoryBytes: discovered.MemoryBytes,
		StorageBytes: discovered.StorageBytes, ComputeSpeedup: 1, Schedulable: true, Metadata: metadata}
	if err := s.resources.Upsert(ctx, resource); err != nil {
		return domain.ResourceSnapshot{}, fmt.Errorf("upsert discovered login node %q: %w", discovered.Name, err)
	}
	for _, runtimeID := range runtimeIDs {
		if err := s.resources.UpsertRuntimeBinding(ctx, domain.ResourceRuntimeBinding{ResourceID: resourceID, RuntimeID: runtimeID, Enabled: true,
			Configuration: map[string]any{"executionTarget": "direct", "role": "login"}}); err != nil {
			return domain.ResourceSnapshot{}, fmt.Errorf("bind discovered login node %q: %w", discovered.Name, err)
		}
	}
	snapshot := domain.ResourceSnapshot{ID: provider.NewID("resource-discovery"), ResourceID: resourceID,
		CapturedAt: time.Now().UTC(), Available: available, Metadata: metadata}
	if err := s.resources.CreateSnapshot(ctx, snapshot); err != nil {
		return domain.ResourceSnapshot{}, fmt.Errorf("snapshot discovered login node %q: %w", discovered.Name, err)
	}
	return snapshot, nil
}

func (s *DiscoveryCoordinator) materializeNodes(
	ctx context.Context,
	definition domain.EnvironmentDefinition,
	connection domain.EnvironmentConnection,
	clusterID string,
	observation ports.ConnectionDiscovery,
) ([]domain.ResourceSnapshot, error) {
	if len(observation.Nodes) == 0 {
		return nil, nil
	}
	runtimeIDs := runtimeIDsForConnection(definition, connection.ID)
	partitions := map[string]domain.Resource{}
	for _, resource := range definition.Resources {
		if resource.Type == domain.ResourceHPCPartition {
			partitions[resource.ProviderID] = resource
			partitions[resource.Name] = resource
		}
	}
	now := time.Now().UTC()
	snapshots := make([]domain.ResourceSnapshot, 0, len(observation.Nodes))
	for _, discovered := range observation.Nodes {
		resourceID := discoveredNodeID(connection.ID, discovered.Name)
		var parent *string
		if clusterID != "" {
			parent = &clusterID
		}
		metadata := cloneMetadata(discovered.Metadata)
		metadata["connectionId"] = connection.ID
		metadata["discovered"] = true
		resource := domain.Resource{ID: resourceID, EnvironmentVersionID: definition.Version.ID,
			ParentResourceID: parent, ExecutionTarget: domain.ExecutionTargetBatch,
			Type: domain.ResourceHPCMachine, Name: discovered.Name, ProviderID: discovered.Name,
			Architecture: discovered.Architecture, CPUCores: discovered.CPUCores,
			CPUCapacity: float64(discovered.CPUCores), MemoryBytes: discovered.MemoryBytes,
			StorageBytes: discovered.StorageBytes, ComputeSpeedup: 1, Schedulable: true, Metadata: metadata}
		if err := s.resources.Upsert(ctx, resource); err != nil {
			return nil, fmt.Errorf("upsert discovered SLURM node %q: %w", discovered.Name, err)
		}
		for _, runtimeID := range runtimeIDs {
			if err := s.resources.UpsertRuntimeBinding(ctx, domain.ResourceRuntimeBinding{ResourceID: resourceID, RuntimeID: runtimeID, Enabled: true}); err != nil {
				return nil, fmt.Errorf("bind discovered SLURM node %q: %w", discovered.Name, err)
			}
		}
		for _, partitionName := range discovered.Partitions {
			partition, ok := partitions[partitionName]
			if !ok {
				continue
			}
			if err := s.resources.UpsertRelation(ctx, domain.ResourceRelation{EnvironmentVersionID: definition.Version.ID,
				SourceResourceID: partition.ID, TargetResourceID: resourceID, Type: domain.ResourceRelationContains,
				Metadata: map[string]any{"discovered": true}}); err != nil {
				return nil, fmt.Errorf("relate partition %q to node %q: %w", partitionName, discovered.Name, err)
			}
		}
		snapshot := domain.ResourceSnapshot{ID: provider.NewID("resource-discovery"), ResourceID: resourceID,
			CapturedAt: now, Available: slurmNodeAvailable(discovered.State), Metadata: metadata}
		if err := s.resources.CreateSnapshot(ctx, snapshot); err != nil {
			return nil, fmt.Errorf("snapshot discovered SLURM node %q: %w", discovered.Name, err)
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

func clusterResourceID(connectionID string) string { return connectionID + "-cluster" }
func loginResourceID(connectionID string) string   { return connectionID + "-login" }

func clusterResourceName(definition domain.EnvironmentDefinition, connection domain.EnvironmentConnection) string {
	if strings.TrimSpace(definition.Environment.Name) != "" {
		return definition.Environment.Name + " cluster"
	}
	return connection.Name + " cluster"
}

func minPositive(value, limit int) int {
	if value <= 0 || value > limit {
		return limit
	}
	return value
}

func minPositive64(value, limit int64) int64 {
	if value <= 0 || value > limit {
		return limit
	}
	return value
}

func runtimeIDsForConnection(definition domain.EnvironmentDefinition, connectionID string) []string {
	result := []string{}
	for _, runtime := range definition.Runtimes {
		if configuredConnectionID(runtime.Configuration) == connectionID {
			result = append(result, runtime.ID)
		}
	}
	return result
}

func discoveredNodeID(connectionID, nodeName string) string {
	replacer := strings.NewReplacer("/", "-", " ", "-", ":", "-")
	return replacer.Replace(connectionID + "-node-" + nodeName)
}

func slurmNodeAvailable(state string) bool {
	state = strings.ToLower(strings.TrimRight(strings.TrimSpace(state), "*~#$+"))
	for _, unavailable := range []string{"down", "drain", "drained", "fail", "failing", "unknown"} {
		if state == unavailable {
			return false
		}
	}
	return state != ""
}

func (s *DiscoveryCoordinator) findConnection(ctx context.Context, id string) (*domain.EnvironmentDefinition, domain.EnvironmentConnection, error) {
	definitions, err := s.catalog.List(ctx)
	if err != nil {
		return nil, domain.EnvironmentConnection{}, err
	}
	for index := range definitions {
		for _, connection := range definitions[index].Connections {
			if connection.ID == id {
				return &definitions[index], connection, nil
			}
		}
	}
	return nil, domain.EnvironmentConnection{}, fmt.Errorf("environment connection %q was not found", id)
}

func boundResourceIDs(definition domain.EnvironmentDefinition, connectionID string) []string {
	runtimeIDs := map[string]bool{}
	for _, runtime := range definition.Runtimes {
		if configuredConnectionID(runtime.Configuration) == connectionID {
			runtimeIDs[runtime.ID] = true
		}
	}
	ids := map[string]bool{}
	for _, binding := range definition.RuntimeBindings {
		if binding.Enabled && runtimeIDs[binding.RuntimeID] {
			ids[binding.ResourceID] = true
		}
	}
	result := make([]string, 0, len(ids))
	for _, resource := range definition.Resources {
		if ids[resource.ID] {
			result = append(result, resource.ID)
		}
	}
	return result
}

func configuredConnectionID(configuration map[string]any) string {
	value, _ := configuration["connectionId"].(string)
	return value
}
func cloneMetadata(input map[string]any) map[string]any {
	output := map[string]any{}
	for key, value := range input {
		output[key] = value
	}
	return output
}
