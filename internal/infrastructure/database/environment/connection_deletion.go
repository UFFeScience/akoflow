package environment

import (
	"context"
	"database/sql"

	"github.com/UFFeScience/akoflow/internal/application/ports"
)

var _ ports.ConnectionDeletionStore = (*Repository)(nil)

func (r *Repository) PreviewConnectionDeletion(ctx context.Context, id string) (ports.ConnectionDeletion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.ConnectionDeletion{}, err
	}
	defer func() { _ = tx.Rollback() }()
	return connectionDeletion(ctx, tx, id)
}

func (r *Repository) DeleteConnection(ctx context.Context, id string) (ports.ConnectionDeletion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ports.ConnectionDeletion{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Take the write lock before inspecting dependencies.
	if _, err := tx.ExecContext(ctx, "UPDATE environment_connections SET name=name WHERE id=?", id); err != nil {
		return ports.ConnectionDeletion{}, err
	}
	deletion, err := connectionDeletion(ctx, tx, id)
	if err != nil || len(deletion.Dependencies) > 0 {
		return deletion, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM environment_connections WHERE id=?", id); err != nil {
		return deletion, err
	}
	return deletion, tx.Commit()
}

func connectionDeletion(ctx context.Context, tx *sql.Tx, id string) (ports.ConnectionDeletion, error) {
	deletion := ports.ConnectionDeletion{Dependencies: []ports.ConnectionDependency{}}
	err := tx.QueryRowContext(ctx, "SELECT name FROM environment_connections WHERE id=?", id).Scan(&deletion.Name)
	if err != nil {
		return deletion, err
	}
	rows, err := tx.QueryContext(ctx, connectionDependencies, id)
	if err != nil {
		return deletion, err
	}
	defer rows.Close()
	for rows.Next() {
		var item ports.ConnectionDependency
		if err := rows.Scan(&item.Kind, &item.ID, &item.Name); err != nil {
			return deletion, err
		}
		deletion.Dependencies = append(deletion.Dependencies, item)
	}
	return deletion, rows.Err()
}

const connectionDependencies = `
WITH runtimes AS (
	SELECT id, name FROM environment_runtimes WHERE json_extract(configuration, '$.connectionId')=?1
), resources_using_connection AS (
	SELECT id, name FROM resources WHERE json_extract(metadata, '$.connectionId')=?1
), execution_resources AS (
	SELECT id FROM resources_using_connection
	UNION SELECT resource_id FROM resource_runtime_bindings WHERE enabled=1 AND runtime_id IN (SELECT id FROM runtimes)
)
SELECT 'runtime' AS kind, id, name FROM runtimes
UNION SELECT 'resource', id, name FROM resources_using_connection
UNION SELECT 'storage', id, name FROM storage_resources
	WHERE json_extract(configuration, '$.connectionId')=?1 OR json_extract(metadata, '$.connectionId')=?1
UNION SELECT 'execution', er.id, er.id FROM execution_runs er
	WHERE er.status IN ('created', 'running') AND (
		EXISTS (SELECT 1 FROM activity_workspaces aw WHERE aw.execution_run_id=er.id AND aw.connection_id=?1)
		OR EXISTS (SELECT 1 FROM schedule_plan_assignments spa JOIN execution_resources r ON r.id=spa.resource_id
			WHERE spa.schedule_plan_id=er.schedule_plan_id)
	)
UNION SELECT 'console command', id, id FROM console_commands WHERE connection_id=?1 AND status='running'
UNION SELECT 'console session', id, id FROM console_sessions WHERE connection_id=?1 AND status IN ('starting','connected')
UNION SELECT 'cloud capacity target', t.id, t.name
	FROM cloud_capacity_targets t WHERE ?1=(
		SELECT c.id FROM environment_connections c
		WHERE c.environment_id=t.environment_id AND c.type='cloud' ORDER BY c.name LIMIT 1
	)
ORDER BY kind, name, id`
