package resource

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

const (
	StatusReady    = true
	StatusNotReady = false
)

type Repository struct{ db *sql.DB }

var _ ports.ResourceInventory = (*Repository)(nil)

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Upsert(ctx context.Context, resource domain.Resource) error {
	metadata, err := json.Marshal(resource.Metadata)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO resources (
		id, environment_version_id, execution_target, parent_resource_id, type, name,
		provider_id, tier, region, zone, architecture, cpu_cores, cpu_capacity,
		memory_bytes, storage_bytes, compute_speedup, price_per_second,
		boot_overhead_seconds, container_overhead_seconds, schedulable, metadata
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		execution_target=excluded.execution_target, parent_resource_id=excluded.parent_resource_id,
		type=excluded.type, name=excluded.name, provider_id=excluded.provider_id,
		tier=excluded.tier, region=excluded.region, zone=excluded.zone,
		architecture=excluded.architecture, cpu_cores=excluded.cpu_cores,
		cpu_capacity=excluded.cpu_capacity, memory_bytes=excluded.memory_bytes,
		storage_bytes=excluded.storage_bytes, compute_speedup=excluded.compute_speedup,
		price_per_second=excluded.price_per_second,
		boot_overhead_seconds=excluded.boot_overhead_seconds,
		container_overhead_seconds=excluded.container_overhead_seconds,
		schedulable=excluded.schedulable, metadata=excluded.metadata`,
		resource.ID, resource.EnvironmentVersionID, normalizedExecutionTarget(resource.ExecutionTarget), resource.ParentResourceID,
		resource.Type, resource.Name, resource.ProviderID, resource.Tier, resource.Region,
		resource.Zone, resource.Architecture, resource.CPUCores, resource.CPUCapacity,
		resource.MemoryBytes, resource.StorageBytes, resource.ComputeSpeedup,
		resource.PricePerSecond, resource.BootOverheadSeconds, resource.ContainerOverhead,
		resource.Schedulable, string(metadata),
	)
	return err
}

func (r *Repository) UpsertRuntimeBinding(ctx context.Context, binding domain.ResourceRuntimeBinding) error {
	configuration, err := json.Marshal(binding.Configuration)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO resource_runtime_bindings
		(resource_id, runtime_id, enabled, configuration) VALUES (?, ?, ?, ?)
		ON CONFLICT(resource_id, runtime_id) DO UPDATE SET
		enabled=excluded.enabled, configuration=excluded.configuration`,
		binding.ResourceID, binding.RuntimeID, binding.Enabled, string(configuration))
	return err
}

func (r *Repository) UpsertRelation(ctx context.Context, relation domain.ResourceRelation) error {
	metadata, err := json.Marshal(relation.Metadata)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO resource_relations
		(environment_version_id, source_resource_id, target_resource_id, relation_type, metadata)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(source_resource_id, target_resource_id, relation_type) DO UPDATE SET
		environment_version_id=excluded.environment_version_id, metadata=excluded.metadata`,
		relation.EnvironmentVersionID, relation.SourceResourceID, relation.TargetResourceID,
		relation.Type, string(metadata))
	return err
}

const resourceColumns = `id, environment_version_id, execution_target, parent_resource_id,
	type, name, provider_id, tier, region, zone, architecture, cpu_cores, cpu_capacity,
	memory_bytes, storage_bytes, compute_speedup, price_per_second,
	boot_overhead_seconds, container_overhead_seconds, schedulable, metadata`

func scanResource(scanner interface{ Scan(...any) error }) (*domain.Resource, error) {
	var resource domain.Resource
	var parent sql.NullString
	var resourceType string
	var executionTarget string
	var schedulable bool
	var metadata string
	if err := scanner.Scan(
		&resource.ID, &resource.EnvironmentVersionID, &executionTarget, &parent,
		&resourceType, &resource.Name, &resource.ProviderID, &resource.Tier,
		&resource.Region, &resource.Zone, &resource.Architecture, &resource.CPUCores,
		&resource.CPUCapacity, &resource.MemoryBytes, &resource.StorageBytes,
		&resource.ComputeSpeedup, &resource.PricePerSecond, &resource.BootOverheadSeconds,
		&resource.ContainerOverhead, &schedulable, &metadata,
	); err != nil {
		return nil, err
	}
	resource.Type = domain.ResourceType(resourceType)
	resource.ExecutionTarget = domain.ResourceExecutionTarget(executionTarget)
	resource.Schedulable = schedulable
	if parent.Valid {
		resource.ParentResourceID = &parent.String
	}
	if metadata != "" {
		if err := json.Unmarshal([]byte(metadata), &resource.Metadata); err != nil {
			return nil, fmt.Errorf("decode resource metadata: %w", err)
		}
	}
	return &resource, nil
}

func normalizedExecutionTarget(target domain.ResourceExecutionTarget) domain.ResourceExecutionTarget {
	if target == "" {
		return domain.ExecutionTargetBatch
	}
	return target
}

func (r *Repository) FindByID(ctx context.Context, id string) (*domain.Resource, error) {
	resource, err := scanResource(r.db.QueryRowContext(ctx, `SELECT `+resourceColumns+` FROM resources WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return resource, err
}

func (r *Repository) FindByProviderID(ctx context.Context, environmentVersionID, providerID string) (*domain.Resource, error) {
	resource, err := scanResource(r.db.QueryRowContext(ctx, `SELECT `+resourceColumns+` FROM resources WHERE environment_version_id = ? AND provider_id = ?`, environmentVersionID, providerID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return resource, err
}

func (r *Repository) list(ctx context.Context, query string, args ...any) ([]domain.Resource, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	resources := make([]domain.Resource, 0)
	for rows.Next() {
		resource, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		resources = append(resources, *resource)
	}
	return resources, rows.Err()
}

func (r *Repository) List(ctx context.Context) ([]domain.Resource, error) {
	return r.list(ctx, `SELECT `+resourceColumns+` FROM resources ORDER BY environment_version_id, name`)
}

func (r *Repository) ListByRuntime(ctx context.Context, environmentVersionID, runtimeID string) ([]domain.Resource, error) {
	return r.list(ctx, `SELECT `+prefixedResourceColumns("r")+` FROM resources r
		JOIN resource_runtime_bindings b ON b.resource_id=r.id AND b.enabled=1
		WHERE r.environment_version_id = ? AND b.runtime_id = ?`, environmentVersionID, runtimeID)
}

func prefixedResourceColumns(prefix string) string {
	return prefix + ".id, " + prefix + ".environment_version_id, " + prefix + ".execution_target, " +
		prefix + ".parent_resource_id, " + prefix + ".type, " + prefix + ".name, " +
		prefix + ".provider_id, " + prefix + ".tier, " + prefix + ".region, " +
		prefix + ".zone, " + prefix + ".architecture, " + prefix + ".cpu_cores, " +
		prefix + ".cpu_capacity, " + prefix + ".memory_bytes, " + prefix + ".storage_bytes, " +
		prefix + ".compute_speedup, " + prefix + ".price_per_second, " +
		prefix + ".boot_overhead_seconds, " + prefix + ".container_overhead_seconds, " +
		prefix + ".schedulable, " + prefix + ".metadata"
}

func (r *Repository) ListSchedulable(ctx context.Context, environmentVersionID string) ([]domain.Resource, error) {
	return r.list(ctx, `SELECT `+resourceColumns+` FROM resources WHERE environment_version_id = ? AND schedulable = 1`, environmentVersionID)
}

func (r *Repository) CreateSnapshot(ctx context.Context, snapshot domain.ResourceSnapshot) error {
	metadata, err := json.Marshal(snapshot.Metadata)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO resource_snapshots (
		id, resource_id, captured_at, available, cpu_used, memory_used_bytes,
		network_in_bps, network_out_bps, disk_read_bps, disk_write_bps, queue_length, metadata
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, snapshot.ID, snapshot.ResourceID,
		snapshot.CapturedAt, snapshot.Available, snapshot.CPUUsed, snapshot.MemoryUsedBytes,
		snapshot.NetworkInBPS, snapshot.NetworkOutBPS, snapshot.DiskReadBPS,
		snapshot.DiskWriteBPS, snapshot.QueueLength, string(metadata))
	return err
}

func (r *Repository) LatestSnapshot(ctx context.Context, resourceID string) (*domain.ResourceSnapshot, error) {
	var snapshot domain.ResourceSnapshot
	var metadata string
	err := r.db.QueryRowContext(ctx, `SELECT id, resource_id, captured_at, available, cpu_used,
		memory_used_bytes, network_in_bps, network_out_bps, disk_read_bps,
		disk_write_bps, queue_length, metadata FROM resource_snapshots
		WHERE resource_id = ? ORDER BY captured_at DESC LIMIT 1`, resourceID).Scan(
		&snapshot.ID, &snapshot.ResourceID, &snapshot.CapturedAt, &snapshot.Available,
		&snapshot.CPUUsed, &snapshot.MemoryUsedBytes, &snapshot.NetworkInBPS,
		&snapshot.NetworkOutBPS, &snapshot.DiskReadBPS, &snapshot.DiskWriteBPS,
		&snapshot.QueueLength, &metadata,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if metadata != "" {
		if err := json.Unmarshal([]byte(metadata), &snapshot.Metadata); err != nil {
			return nil, err
		}
	}
	return &snapshot, nil
}
