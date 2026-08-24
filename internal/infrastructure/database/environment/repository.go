package environment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type Definition = domain.EnvironmentDefinition
type Repository struct{ db *sql.DB }

var _ ports.EnvironmentCatalog = (*Repository)(nil)
var _ ports.StorageCatalog = (*Repository)(nil)

func New(db *sql.DB) *Repository { return &Repository{db: db} }

// Replace updates an environment through the same complete definition used at
// creation time. It is transactional: if the edited inventory cannot be
// stored, the previous definition remains intact.
func (r *Repository) Replace(ctx context.Context, definition Definition) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM environments WHERE id=?)`, definition.Environment.ID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}
	for _, statement := range []string{
		`DELETE FROM resource_snapshots WHERE resource_id IN (SELECT r.id FROM resources r JOIN environment_versions ev ON ev.id=r.environment_version_id WHERE ev.environment_id=?)`,
		`DELETE FROM activity_resource_profiles WHERE resource_id IN (SELECT r.id FROM resources r JOIN environment_versions ev ON ev.id=r.environment_version_id WHERE ev.environment_id=?)`,
		`DELETE FROM resource_relations WHERE environment_version_id IN (SELECT id FROM environment_versions WHERE environment_id=?)`,
		`DELETE FROM discovery_runs WHERE environment_version_id IN (SELECT id FROM environment_versions WHERE environment_id=?)`,
		`DELETE FROM storage_resources WHERE environment_version_id IN (SELECT id FROM environment_versions WHERE environment_id=?)`,
		`DELETE FROM resources WHERE environment_version_id IN (SELECT id FROM environment_versions WHERE environment_id=?)`,
		`DELETE FROM environment_runtimes WHERE environment_version_id IN (SELECT id FROM environment_versions WHERE environment_id=?)`,
		`DELETE FROM environment_connections WHERE environment_id=?`,
		`DELETE FROM environment_versions WHERE environment_id=?`,
	} {
		if _, err := tx.ExecContext(ctx, statement, definition.Environment.ID); err != nil {
			return fmt.Errorf("environment %q cannot be replaced while it is in use: %w", definition.Environment.ID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE environments SET name=?, description=?, status=? WHERE id=?`, definition.Environment.Name, definition.Environment.Description, definition.Environment.Status, definition.Environment.ID); err != nil {
		return err
	}
	if err := insertConnections(tx, definition); err != nil {
		return err
	}
	if err := insertEnvironmentVersion(tx, definition); err != nil {
		return err
	}
	if err := insertRuntimes(tx, definition); err != nil {
		return err
	}
	if err := insertResources(tx, definition); err != nil {
		return err
	}
	if err := insertRuntimeBindings(tx, definition); err != nil {
		return err
	}
	if err := insertResourceRelations(tx, definition); err != nil {
		return err
	}
	if err := insertStorages(tx, definition); err != nil {
		return err
	}
	if err := insertProfiles(tx, definition); err != nil {
		return err
	}
	return tx.Commit()
}

// Delete removes an environment and its discovered inventory. Environments that
// are already part of a scope, a plan, or an execution must remain immutable so
// existing plans keep their reproducible infrastructure snapshot.
func (r *Repository) Delete(ctx context.Context, environmentID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM environments WHERE id=?)`, environmentID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}

	for _, dependency := range []struct {
		name  string
		query string
	}{
		{"execution scope", `SELECT EXISTS(SELECT 1 FROM execution_scope_environments ese JOIN environment_versions ev ON ev.id=ese.environment_version_id WHERE ev.environment_id=?)`},
		{"network topology", `SELECT EXISTS(SELECT 1 FROM network_links nl JOIN resources r ON r.id=nl.source_resource_id OR r.id=nl.target_resource_id JOIN environment_versions ev ON ev.id=r.environment_version_id WHERE ev.environment_id=?)`},
		{"schedule plan", `SELECT EXISTS(SELECT 1 FROM schedule_plan_assignments spa JOIN resources r ON r.id=spa.resource_id JOIN environment_versions ev ON ev.id=r.environment_version_id WHERE ev.environment_id=?)`},
		{"execution data", `SELECT EXISTS(SELECT 1 FROM data_locations dl LEFT JOIN resources r ON r.id=dl.resource_id LEFT JOIN storage_resources sr ON sr.id=dl.storage_resource_id LEFT JOIN environment_versions rv ON rv.id=r.environment_version_id LEFT JOIN environment_versions sv ON sv.id=sr.environment_version_id WHERE rv.environment_id=? OR sv.environment_id=?)`},
		{"transfer observation", `SELECT EXISTS(SELECT 1 FROM data_transfer_observations dto JOIN resources r ON r.id=dto.source_resource_id OR r.id=dto.target_resource_id JOIN environment_versions ev ON ev.id=r.environment_version_id WHERE ev.environment_id=?)`},
		{"console activity", `SELECT EXISTS(SELECT 1 FROM console_commands cc JOIN resources r ON r.id=cc.resource_id JOIN environment_versions ev ON ev.id=r.environment_version_id WHERE ev.environment_id=?) OR EXISTS(SELECT 1 FROM console_sessions cs JOIN resources r ON r.id=cs.resource_id JOIN environment_versions ev ON ev.id=r.environment_version_id WHERE ev.environment_id=?)`},
		{"simulation scenario", `SELECT EXISTS(SELECT 1 FROM simulation_scenarios ss JOIN environment_versions ev ON ev.id=ss.environment_version_id WHERE ev.environment_id=?)`},
	} {
		args := []any{environmentID}
		if dependency.name == "execution data" || dependency.name == "console activity" {
			args = []any{environmentID, environmentID}
		}
		if err := tx.QueryRowContext(ctx, dependency.query, args...).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("environment %q cannot be deleted because it is used by a %s", environmentID, dependency.name)
		}
	}

	statements := []string{
		`DELETE FROM resource_snapshots WHERE resource_id IN (SELECT r.id FROM resources r JOIN environment_versions ev ON ev.id=r.environment_version_id WHERE ev.environment_id=?)`,
		`DELETE FROM activity_resource_profiles WHERE resource_id IN (SELECT r.id FROM resources r JOIN environment_versions ev ON ev.id=r.environment_version_id WHERE ev.environment_id=?)`,
		`DELETE FROM resource_relations WHERE environment_version_id IN (SELECT id FROM environment_versions WHERE environment_id=?)`,
		`DELETE FROM discovery_runs WHERE environment_version_id IN (SELECT id FROM environment_versions WHERE environment_id=?)`,
		`DELETE FROM storage_resources WHERE environment_version_id IN (SELECT id FROM environment_versions WHERE environment_id=?)`,
		`DELETE FROM resources WHERE environment_version_id IN (SELECT id FROM environment_versions WHERE environment_id=?)`,
		`DELETE FROM environment_runtimes WHERE environment_version_id IN (SELECT id FROM environment_versions WHERE environment_id=?)`,
		`DELETE FROM environment_connections WHERE environment_id=?`,
		`DELETE FROM environment_versions WHERE environment_id=?`,
		`DELETE FROM environments WHERE id=?`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement, environmentID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) List(ctx context.Context) ([]domain.EnvironmentDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM environments ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	definitions := make([]domain.EnvironmentDefinition, 0, len(ids))
	for _, id := range ids {
		definition, err := r.Find(ctx, id)
		if err != nil {
			return nil, err
		}
		if definition != nil {
			definitions = append(definitions, *definition)
		}
	}
	return definitions, nil
}

func (r *Repository) Find(ctx context.Context, id string) (*domain.EnvironmentDefinition, error) {
	var definition domain.EnvironmentDefinition
	err := r.db.QueryRowContext(ctx, `SELECT id, name, description, status, created_at
		FROM environments WHERE id=?`, id).Scan(&definition.Environment.ID,
		&definition.Environment.Name, &definition.Environment.Description,
		&definition.Environment.Status, &definition.Environment.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var publishedAt sql.NullTime
	err = r.db.QueryRowContext(ctx, `SELECT id, environment_id, version, status,
		network_model, interference_model, cost_model, configuration_hash,
		created_at, published_at FROM environment_versions WHERE environment_id=?
		ORDER BY version DESC LIMIT 1`, id).Scan(&definition.Version.ID,
		&definition.Version.EnvironmentID, &definition.Version.Version,
		&definition.Version.Status, &definition.Version.NetworkModel,
		&definition.Version.InterferenceModel, &definition.Version.CostModel,
		&definition.Version.ConfigurationHash, &definition.Version.CreatedAt, &publishedAt)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if publishedAt.Valid {
		definition.Version.PublishedAt = &publishedAt.Time
	}
	definition.Connections, err = r.ListConnections(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, connection := range definition.Connections {
		checks, checkErr := r.ListConnectionChecks(ctx, connection.ID, 20)
		if checkErr != nil {
			return nil, checkErr
		}
		definition.ConnectionChecks = append(definition.ConnectionChecks, checks...)
	}
	if definition.Version.ID == "" {
		return &definition, nil
	}
	definition.Runtimes, err = r.listRuntimes(ctx, definition.Version.ID)
	if err != nil {
		return nil, err
	}
	definition.Resources, err = r.listResources(ctx, definition.Version.ID)
	if err != nil {
		return nil, err
	}
	definition.RuntimeBindings, err = r.listRuntimeBindings(ctx, definition.Version.ID)
	if err != nil {
		return nil, err
	}
	definition.Relations, err = r.listResourceRelations(ctx, definition.Version.ID)
	if err != nil {
		return nil, err
	}
	definition.Storages, err = r.listStorages(ctx, definition.Version.ID, definition.Runtimes)
	if err != nil {
		return nil, err
	}
	definition.Profiles, err = r.listProfiles(ctx, definition.Version.ID)
	if err != nil {
		return nil, err
	}
	return &definition, nil
}

func (r *Repository) listResourceRelations(
	ctx context.Context,
	versionID string,
) ([]domain.ResourceRelation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT environment_version_id,
		source_resource_id, target_resource_id, relation_type, metadata
		FROM resource_relations WHERE environment_version_id=?
		ORDER BY source_resource_id, target_resource_id`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.ResourceRelation, 0)
	for rows.Next() {
		var item domain.ResourceRelation
		var metadata string
		if err := rows.Scan(
			&item.EnvironmentVersionID,
			&item.SourceResourceID,
			&item.TargetResourceID,
			&item.Type,
			&metadata,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(metadata), &item.Metadata); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) listStorages(
	ctx context.Context,
	versionID string,
	runtimes []domain.EnvironmentRuntime,
) ([]domain.StorageResource, error) {
	items := make(map[string]domain.StorageResource)
	for _, runtime := range runtimes {
		values, err := r.ListRuntimeStorages(ctx, versionID, runtime.ID)
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			existing, found := items[value.ID]
			if found {
				existing.RuntimeBindings = append(existing.RuntimeBindings, value.RuntimeBindings...)
				items[value.ID] = existing
				continue
			}
			items[value.ID] = value
		}
	}
	result := make([]domain.StorageResource, 0, len(items))
	for _, value := range items {
		result = append(result, value)
	}
	return result, nil
}

func (r *Repository) listProfiles(
	ctx context.Context,
	versionID string,
) ([]domain.ActivityResourceProfile, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT p.id, p.activity_type_id, p.resource_id,
		p.runtime_seconds, p.runtime_stddev_seconds, p.cpu_utilization,
		p.peak_memory_bytes, p.disk_read_bytes, p.disk_write_bytes, p.energy_joules,
		p.source, p.sample_size, p.model_version, p.metadata
		FROM activity_resource_profiles p
		JOIN resources r ON r.id=p.resource_id
		WHERE r.environment_version_id=? ORDER BY p.activity_type_id, p.resource_id`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := make([]domain.ActivityResourceProfile, 0)
	for rows.Next() {
		var profile domain.ActivityResourceProfile
		var metadata string
		if err := rows.Scan(
			&profile.ID,
			&profile.ActivityTypeID,
			&profile.ResourceID,
			&profile.RuntimeSeconds,
			&profile.RuntimeStdDevSeconds,
			&profile.CPUUtilization,
			&profile.PeakMemoryBytes,
			&profile.DiskReadBytes,
			&profile.DiskWriteBytes,
			&profile.EnergyJoules,
			&profile.Source,
			&profile.SampleSize,
			&profile.ModelVersion,
			&metadata,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(metadata), &profile.Metadata); err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (r *Repository) listRuntimes(ctx context.Context, versionID string) ([]domain.EnvironmentRuntime, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT r.environment_version_id, r.id, r.name,
		r.driver, r.mode, r.role, r.configuration, c.capabilities FROM environment_runtimes r
		LEFT JOIN environment_runtime_capabilities c ON c.runtime_id=r.id
		WHERE r.environment_version_id=? ORDER BY r.name`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.EnvironmentRuntime, 0)
	for rows.Next() {
		var item domain.EnvironmentRuntime
		var configuration, capabilities string
		if err := rows.Scan(&item.EnvironmentVersionID, &item.ID, &item.Name,
			&item.Driver, &item.Mode, &item.Role,
			&configuration, &capabilities); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(configuration), &item.Configuration); err != nil {
			return nil, err
		}
		if capabilities != "" {
			if err := json.Unmarshal([]byte(capabilities), &item.Capabilities); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) listResources(ctx context.Context, versionID string) ([]domain.Resource, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, environment_version_id,
		execution_target, parent_resource_id, type, name, provider_id, tier, region,
		zone, architecture, cpu_cores, cpu_capacity, memory_bytes, storage_bytes,
		compute_speedup, price_per_second, boot_overhead_seconds,
		container_overhead_seconds, schedulable, metadata FROM resources
		WHERE environment_version_id=? ORDER BY name`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Resource, 0)
	for rows.Next() {
		var item domain.Resource
		var parent sql.NullString
		var metadata string
		if err := rows.Scan(&item.ID, &item.EnvironmentVersionID,
			&item.ExecutionTarget, &parent, &item.Type, &item.Name, &item.ProviderID,
			&item.Tier, &item.Region, &item.Zone, &item.Architecture, &item.CPUCores,
			&item.CPUCapacity, &item.MemoryBytes, &item.StorageBytes,
			&item.ComputeSpeedup, &item.PricePerSecond, &item.BootOverheadSeconds,
			&item.ContainerOverhead, &item.Schedulable, &metadata); err != nil {
			return nil, err
		}
		if parent.Valid {
			item.ParentResourceID = &parent.String
		}
		if err := json.Unmarshal([]byte(metadata), &item.Metadata); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) Create(ctx context.Context, definition Definition) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertEnvironment(tx, definition); err != nil {
		return err
	}
	if err := insertConnections(tx, definition); err != nil {
		return err
	}
	if err := insertEnvironmentVersion(tx, definition); err != nil {
		return err
	}
	if err := insertRuntimes(tx, definition); err != nil {
		return err
	}
	if err := insertResources(tx, definition); err != nil {
		return err
	}
	if err := insertRuntimeBindings(tx, definition); err != nil {
		return err
	}
	if err := insertResourceRelations(tx, definition); err != nil {
		return err
	}
	if err := insertStorages(tx, definition); err != nil {
		return err
	}
	if err := insertProfiles(tx, definition); err != nil {
		return err
	}
	return tx.Commit()
}

func insertResourceRelations(tx *sql.Tx, definition Definition) error {
	for _, relation := range definition.Relations {
		metadata, err := json.Marshal(relation.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO resource_relations (
			environment_version_id, source_resource_id, target_resource_id,
			relation_type, metadata
		) VALUES (?, ?, ?, ?, ?)`, definition.Version.ID, relation.SourceResourceID,
			relation.TargetResourceID, relation.Type, string(metadata)); err != nil {
			return err
		}
	}
	return nil
}

func insertStorages(tx *sql.Tx, definition Definition) error {
	for _, storage := range definition.Storages {
		configuration, err := json.Marshal(storage.Configuration)
		if err != nil {
			return err
		}
		metadata, err := json.Marshal(storage.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO storage_resources (
			id, environment_version_id, name, type, endpoint, capacity_bytes,
			shared, read_only, configuration, credential_reference, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, storage.ID, definition.Version.ID,
			storage.Name, storage.Type, storage.Endpoint, storage.CapacityBytes,
			storage.Shared, storage.ReadOnly, string(configuration),
			storage.CredentialReference, string(metadata)); err != nil {
			return err
		}
		for _, binding := range storage.RuntimeBindings {
			bindingConfiguration, err := json.Marshal(binding.Configuration)
			if err != nil {
				return err
			}
			containerPath := binding.ContainerPath
			if containerPath == "" {
				containerPath = "/akoflow/data"
			}
			if _, err := tx.Exec(`INSERT INTO storage_runtime_bindings (
				storage_resource_id, environment_version_id, runtime_id, is_default,
				host_path, container_path, read_only, configuration
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, storage.ID, definition.Version.ID,
				binding.RuntimeID, binding.Default, binding.HostPath, containerPath,
				binding.ReadOnly, string(bindingConfiguration)); err != nil {
				return err
			}
		}
	}
	return nil
}

func insertEnvironment(tx *sql.Tx, definition Definition) error {
	environment := definition.Environment
	if environment.Status == "" {
		environment.Status = domain.EnvironmentDefined
	}
	_, err := tx.Exec(`INSERT INTO environments (id, name, description, status)
		VALUES (?, ?, ?, ?)`, environment.ID, environment.Name, environment.Description, environment.Status)
	return err
}

func insertConnections(tx *sql.Tx, definition Definition) error {
	environmentID := definition.Environment.ID
	for _, connection := range definition.Connections {
		configuration, err := json.Marshal(connection.Configuration)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO environment_connections (
			id, environment_id, name, type, endpoint, username, credential_ref, configuration
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, connection.ID, environmentID,
			connection.Name, connection.Type, connection.Endpoint, connection.Username,
			connection.CredentialRef, string(configuration))
		if err != nil {
			return err
		}
	}
	return nil
}

func insertEnvironmentVersion(tx *sql.Tx, definition Definition) error {
	version := definition.Version
	_, err := tx.Exec(`INSERT INTO environment_versions (
		id, environment_id, version, status, network_model, interference_model,
		cost_model, configuration_hash, published_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, version.ID, definition.Environment.ID,
		version.Version, version.Status, version.NetworkModel, version.InterferenceModel,
		version.CostModel, version.ConfigurationHash,
		version.PublishedAt)
	return err
}

func insertRuntimes(tx *sql.Tx, definition Definition) error {
	versionID := definition.Version.ID
	for _, runtime := range definition.Runtimes {
		configuration, err := json.Marshal(runtime.Configuration)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO environment_runtimes (
			id, environment_version_id, name, driver, mode, role, configuration
		) VALUES (?, ?, ?, ?, ?, ?, ?)`, runtime.ID, versionID, runtime.Name,
			runtime.Driver, runtime.Mode, runtime.Role,
			string(configuration)); err != nil {
			return err
		}
		capabilities, err := json.Marshal(runtime.Capabilities)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO environment_runtime_capabilities (
			runtime_id, capabilities
		) VALUES (?, ?)`, runtime.ID, string(capabilities)); err != nil {
			return err
		}
	}
	return nil
}

func insertResources(tx *sql.Tx, definition Definition) error {
	for _, resource := range definition.Resources {
		metadata, err := json.Marshal(resource.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO resources (
			id, environment_version_id, execution_target, parent_resource_id, type, name,
			provider_id, tier, region, zone, architecture, cpu_cores, cpu_capacity,
			memory_bytes, storage_bytes, compute_speedup, price_per_second,
			boot_overhead_seconds, container_overhead_seconds, schedulable, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			resource.ID, definition.Version.ID, normalizedExecutionTarget(resource.ExecutionTarget), resource.ParentResourceID,
			resource.Type, resource.Name, resource.ProviderID, resource.Tier,
			resource.Region, resource.Zone, resource.Architecture, resource.CPUCores,
			resource.CPUCapacity, resource.MemoryBytes, resource.StorageBytes,
			resource.ComputeSpeedup, resource.PricePerSecond,
			resource.BootOverheadSeconds, resource.ContainerOverhead,
			resource.Schedulable, string(metadata)); err != nil {
			return err
		}
	}
	return nil
}

func insertRuntimeBindings(tx *sql.Tx, definition Definition) error {
	for _, binding := range definition.RuntimeBindings {
		configuration, err := json.Marshal(binding.Configuration)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO resource_runtime_bindings (
			resource_id, runtime_id, enabled, configuration
		) VALUES (?, ?, ?, ?)`, binding.ResourceID, binding.RuntimeID,
			binding.Enabled, string(configuration)); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) listRuntimeBindings(ctx context.Context, versionID string) ([]domain.ResourceRuntimeBinding, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT b.resource_id, b.runtime_id,
		b.enabled, b.configuration FROM resource_runtime_bindings b
		JOIN resources r ON r.id=b.resource_id WHERE r.environment_version_id=?
		ORDER BY b.resource_id, b.runtime_id`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bindings := make([]domain.ResourceRuntimeBinding, 0)
	for rows.Next() {
		var binding domain.ResourceRuntimeBinding
		var configuration string
		if err := rows.Scan(&binding.ResourceID, &binding.RuntimeID,
			&binding.Enabled, &configuration); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(configuration), &binding.Configuration); err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

func normalizedExecutionTarget(target domain.ResourceExecutionTarget) domain.ResourceExecutionTarget {
	if target == "" {
		return domain.ExecutionTargetBatch
	}
	return target
}

func insertProfiles(tx *sql.Tx, definition Definition) error {
	for _, profile := range definition.Profiles {
		metadata, err := json.Marshal(profile.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO activity_resource_profiles (
			id, activity_type_id, resource_id, runtime_seconds,
			runtime_stddev_seconds, cpu_utilization, peak_memory_bytes,
			disk_read_bytes, disk_write_bytes, energy_joules, source, sample_size,
			model_version, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, profile.ID,
			profile.ActivityTypeID, profile.ResourceID, profile.RuntimeSeconds,
			profile.RuntimeStdDevSeconds, profile.CPUUtilization,
			profile.PeakMemoryBytes, profile.DiskReadBytes, profile.DiskWriteBytes,
			profile.EnergyJoules, profile.Source, profile.SampleSize,
			profile.ModelVersion, string(metadata)); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id string, status domain.EnvironmentStatus) error {
	result, err := r.db.ExecContext(ctx, `UPDATE environments SET status=? WHERE id=?`, status, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) UpsertConnection(ctx context.Context, connection domain.EnvironmentConnection) error {
	configuration, err := json.Marshal(connection.Configuration)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO environment_connections (
		id, environment_id, name, type, endpoint, username, credential_ref, configuration
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET name=excluded.name, type=excluded.type,
		endpoint=excluded.endpoint, username=excluded.username,
		credential_ref=excluded.credential_ref, configuration=excluded.configuration`,
		connection.ID, connection.EnvironmentID, connection.Name, connection.Type,
		connection.Endpoint, connection.Username, connection.CredentialRef, string(configuration))
	return err
}

func (r *Repository) ListConnections(ctx context.Context, environmentID string) ([]domain.EnvironmentConnection, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, environment_id, name, type,
		endpoint, username, credential_ref, configuration, created_at
		FROM environment_connections WHERE environment_id=? ORDER BY name`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	connections := make([]domain.EnvironmentConnection, 0)
	for rows.Next() {
		var connection domain.EnvironmentConnection
		var connectionType, configuration string
		if err := rows.Scan(&connection.ID, &connection.EnvironmentID, &connection.Name,
			&connectionType, &connection.Endpoint, &connection.Username,
			&connection.CredentialRef, &configuration, &connection.CreatedAt); err != nil {
			return nil, err
		}
		connection.Type = domain.ConnectionType(connectionType)
		if err := json.Unmarshal([]byte(configuration), &connection.Configuration); err != nil {
			return nil, err
		}
		connections = append(connections, connection)
	}
	return connections, rows.Err()
}

func (r *Repository) FindConnection(ctx context.Context, id string) (*domain.EnvironmentConnection, error) {
	var connection domain.EnvironmentConnection
	var connectionType, configuration string
	err := r.db.QueryRowContext(ctx, `SELECT id, environment_id, name, type,
		endpoint, username, credential_ref, configuration, created_at
		FROM environment_connections WHERE id=?`, id).Scan(
		&connection.ID, &connection.EnvironmentID, &connection.Name, &connectionType,
		&connection.Endpoint, &connection.Username, &connection.CredentialRef,
		&configuration, &connection.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	connection.Type = domain.ConnectionType(connectionType)
	if err := json.Unmarshal([]byte(configuration), &connection.Configuration); err != nil {
		return nil, err
	}
	return &connection, nil
}

func (r *Repository) ListAllConnections(ctx context.Context) ([]domain.EnvironmentConnection, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, environment_id, name, type,
		endpoint, username, credential_ref, configuration, created_at
		FROM environment_connections ORDER BY environment_id, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	connections := make([]domain.EnvironmentConnection, 0)
	for rows.Next() {
		var connection domain.EnvironmentConnection
		var connectionType, configuration string
		if err := rows.Scan(&connection.ID, &connection.EnvironmentID, &connection.Name,
			&connectionType, &connection.Endpoint, &connection.Username,
			&connection.CredentialRef, &configuration, &connection.CreatedAt); err != nil {
			return nil, err
		}
		connection.Type = domain.ConnectionType(connectionType)
		if err := json.Unmarshal([]byte(configuration), &connection.Configuration); err != nil {
			return nil, err
		}
		connections = append(connections, connection)
	}
	return connections, rows.Err()
}

func (r *Repository) SaveConnectionCheck(ctx context.Context, check domain.ConnectionCheck) error {
	metadata, err := json.Marshal(check.Metadata)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO environment_connection_checks (
		id, connection_id, status, message, latency_ms, checked_at, metadata
	) VALUES (?, ?, ?, ?, ?, ?, ?)`, check.ID, check.ConnectionID, check.Status,
		check.Message, check.LatencyMS, check.CheckedAt, string(metadata)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM environment_connection_checks
		WHERE connection_id=? AND id NOT IN (
			SELECT id FROM environment_connection_checks WHERE connection_id=?
			ORDER BY checked_at DESC LIMIT 1000
		)`, check.ConnectionID, check.ConnectionID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ListConnectionChecks(
	ctx context.Context,
	connectionID string,
	limit int,
) ([]domain.ConnectionCheck, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, connection_id, status, message,
		latency_ms, checked_at, metadata FROM environment_connection_checks
		WHERE connection_id=? ORDER BY checked_at DESC LIMIT ?`, connectionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	checks := make([]domain.ConnectionCheck, 0)
	for rows.Next() {
		var check domain.ConnectionCheck
		var metadata string
		if err := rows.Scan(&check.ID, &check.ConnectionID, &check.Status, &check.Message,
			&check.LatencyMS, &check.CheckedAt, &metadata); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(metadata), &check.Metadata); err != nil {
			return nil, err
		}
		checks = append(checks, check)
	}
	return checks, rows.Err()
}

func (r *Repository) ListRuntimeStorages(ctx context.Context, versionID, runtimeID string) ([]domain.StorageResource, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT s.id, s.environment_version_id, s.name,
		s.type, s.endpoint, s.capacity_bytes, s.shared, s.read_only, s.configuration,
		s.credential_reference, s.metadata, b.is_default, b.host_path,
		b.container_path, b.read_only, b.configuration
		FROM storage_resources s
		JOIN storage_runtime_bindings b ON b.storage_resource_id=s.id
		WHERE b.environment_version_id=? AND b.runtime_id=?
		ORDER BY b.is_default DESC, s.name`, versionID, runtimeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	storages := make([]domain.StorageResource, 0)
	for rows.Next() {
		var item domain.StorageResource
		var storageType, storageConfiguration, storageMetadata, bindingConfiguration string
		var binding domain.StorageRuntimeBinding
		if err := rows.Scan(&item.ID, &item.EnvironmentVersionID, &item.Name,
			&storageType, &item.Endpoint, &item.CapacityBytes, &item.Shared,
			&item.ReadOnly, &storageConfiguration, &item.CredentialReference,
			&storageMetadata, &binding.Default, &binding.HostPath,
			&binding.ContainerPath, &binding.ReadOnly, &bindingConfiguration); err != nil {
			return nil, err
		}
		item.Type = domain.StorageType(storageType)
		binding.RuntimeID = runtimeID
		if err := json.Unmarshal([]byte(storageConfiguration), &item.Configuration); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(storageMetadata), &item.Metadata); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(bindingConfiguration), &binding.Configuration); err != nil {
			return nil, err
		}
		item.RuntimeBindings = []domain.StorageRuntimeBinding{binding}
		storages = append(storages, item)
	}
	return storages, rows.Err()
}

func (r *Repository) FindDefaultRuntimeStorage(ctx context.Context, versionID, runtimeID string) (*domain.StorageResource, error) {
	storages, err := r.ListRuntimeStorages(ctx, versionID, runtimeID)
	if err != nil {
		return nil, err
	}
	for i := range storages {
		if storages[i].RuntimeBindings[0].Default {
			return &storages[i], nil
		}
	}
	return nil, sql.ErrNoRows
}
func (r *Repository) UpsertDiscoveredStorage(ctx context.Context, value domain.StorageResource) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	configuration, err := json.Marshal(value.Configuration)
	if err != nil {
		return err
	}
	metadata := value.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["discovered"] = true
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	const statement = `INSERT INTO storage_resources(
		id,environment_version_id,name,type,endpoint,capacity_bytes,shared,read_only,
		configuration,credential_reference,metadata
	) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		capacity_bytes=excluded.capacity_bytes,
		configuration=excluded.configuration,
		metadata=excluded.metadata`
	_, err = tx.ExecContext(ctx, statement,
		value.ID, value.EnvironmentVersionID, value.Name, value.Type,
		value.Endpoint, value.CapacityBytes, value.Shared, value.ReadOnly,
		string(configuration), value.CredentialReference, string(encoded),
	)
	if err != nil {
		return err
	}
	for _, binding := range value.RuntimeBindings {
		configuration, err := json.Marshal(binding.Configuration)
		if err != nil {
			return err
		}
		containerPath := binding.ContainerPath
		if containerPath == "" {
			containerPath = "/akoflow/data"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO storage_runtime_bindings (
			storage_resource_id, environment_version_id, runtime_id, is_default,
			host_path, container_path, read_only, configuration
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(storage_resource_id, runtime_id) DO UPDATE SET
			host_path=excluded.host_path, container_path=excluded.container_path,
			read_only=excluded.read_only, configuration=excluded.configuration`,
			value.ID, value.EnvironmentVersionID, binding.RuntimeID, binding.Default,
			binding.HostPath, containerPath, binding.ReadOnly, string(configuration)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
