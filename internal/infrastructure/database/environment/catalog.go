package environment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func (r *Repository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM environments`).Scan(&count)
	return count, err
}

// ListSummaries intentionally omits performance profiles, historical health
// checks, storage manifests and connector configuration. Scope/version filters
// are applied before loading the related inventory, including historical versions.
func (r *Repository) ListSummaries(ctx context.Context, scopeID, versionID string) ([]domain.EnvironmentDefinition, error) {
	filter := `v.version=(SELECT MAX(version) FROM environment_versions WHERE environment_id=e.id) OR v.id IS NULL`
	var args []any
	if scopeID != "" {
		filter = `v.id IN (SELECT environment_version_id FROM execution_scope_environments WHERE execution_scope_id=?)`
		args = append(args, scopeID)
	}
	if versionID != "" {
		if scopeID != "" {
			filter += ` AND v.id=?`
		} else {
			filter = `v.id=?`
		}
		args = append(args, versionID)
	}
	rows, err := r.db.QueryContext(ctx, environmentSummarySelect+` WHERE (`+filter+`) ORDER BY e.name,e.id,v.version`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.EnvironmentDefinition, 0)
	for rows.Next() {
		item, err := scanEnvironmentSummary(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanEnvironmentSummary(row interface{ Scan(...any) error }) (domain.EnvironmentDefinition, error) {
	var item domain.EnvironmentDefinition
	var versionID sql.NullString
	var version sql.NullInt64
	var resources, runtimes, connections, checks, bindings, relations string
	err := row.Scan(&item.Environment.ID, &item.Environment.Name, &item.Environment.Description,
		&item.Environment.Status, &item.Environment.CreatedAt, &versionID, &version,
		&resources, &runtimes, &connections, &checks, &bindings, &relations)
	if err != nil {
		return item, err
	}
	item.Version.ID, item.Version.EnvironmentID = versionID.String, item.Environment.ID
	item.Version.Version = int(version.Int64)
	for _, field := range []struct {
		raw    string
		target any
	}{
		{resources, &item.Resources}, {runtimes, &item.Runtimes},
		{connections, &item.Connections}, {checks, &item.ConnectionChecks},
		{bindings, &item.RuntimeBindings}, {relations, &item.Relations},
	} {
		if err := json.Unmarshal([]byte(field.raw), field.target); err != nil {
			return item, fmt.Errorf("decode environment summary: %w", err)
		}
	}
	return item, nil
}

const environmentSummarySelect = `SELECT e.id,e.name,e.description,e.status,e.created_at,v.id,v.version,
	(SELECT json_group_array(json_object(
		'id',r.id,'environmentVersionId',r.environment_version_id,'executionTarget',r.execution_target,
		'parentResourceId',COALESCE(r.parent_resource_id,''),'type',r.type,'name',r.name,'providerId',r.provider_id,
		'tier',r.tier,'region',r.region,'zone',r.zone,'architecture',r.architecture,'cpuCores',r.cpu_cores,
		'cpuCapacity',r.cpu_capacity,'memoryBytes',r.memory_bytes,'storageBytes',r.storage_bytes,
		'computeSpeedup',r.compute_speedup,'pricePerSecond',r.price_per_second,
		'bootOverheadSeconds',r.boot_overhead_seconds,'containerOverheadSeconds',r.container_overhead_seconds,
		'schedulable',json(CASE WHEN r.schedulable THEN 'true' ELSE 'false' END),'metadata',json(r.metadata)))
	 FROM resources r WHERE r.environment_version_id=v.id),
	(SELECT json_group_array(json_object('id',r.id,'environmentVersionId',r.environment_version_id,
		'name',r.name,'driver',r.driver,'mode',r.mode,'role',r.role))
	 FROM environment_runtimes r WHERE r.environment_version_id=v.id),
	(SELECT json_group_array(json_object('id',c.id,'environmentId',c.environment_id,'name',c.name,'type',c.type,
		'configuration',json_object('provider',json_extract(c.configuration,'$.provider'),
		'region',json_extract(c.configuration,'$.region')))) FROM environment_connections c WHERE c.environment_id=e.id),
	(SELECT json_group_array(json_object('id',h.id,'connectionId',h.connection_id,'status',h.status,
		'checkedAt',strftime('%Y-%m-%dT%H:%M:%fZ',h.checked_at)))
	 FROM environment_connections c JOIN environment_connection_checks h ON h.id=(
		SELECT id FROM environment_connection_checks WHERE connection_id=c.id ORDER BY checked_at DESC,id DESC LIMIT 1)
	 WHERE c.environment_id=e.id),
	(SELECT json_group_array(json_object('resourceId',b.resource_id,'runtimeId',b.runtime_id,
		'enabled',json(CASE WHEN b.enabled THEN 'true' ELSE 'false' END)))
	 FROM resource_runtime_bindings b JOIN resources r ON r.id=b.resource_id WHERE r.environment_version_id=v.id),
	(SELECT json_group_array(json_object('environmentVersionId',r.environment_version_id,
		'sourceResourceId',r.source_resource_id,'targetResourceId',r.target_resource_id,'type',r.relation_type))
	 FROM resource_relations r WHERE r.environment_version_id=v.id)
	FROM environments e LEFT JOIN environment_versions v ON v.environment_id=e.id`
