package network

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func (r *Repository) CreateScope(ctx context.Context, scope domain.ExecutionScope) error {
	if scope.ID == "" || scope.Name == "" || len(scope.EnvironmentVersionIDs) == 0 {
		return fmt.Errorf("execution scope id, name and environments are required")
	}
	metadata, err := json.Marshal(scope.Metadata)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO execution_scopes
		(id, name, network_topology_id, metadata) VALUES (?, ?, NULLIF(?, ''), ?)`,
		scope.ID, scope.Name, scope.NetworkTopologyID, metadata); err != nil {
		return err
	}
	for _, versionID := range scope.EnvironmentVersionIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO execution_scope_environments
			(execution_scope_id, environment_version_id) VALUES (?, ?)`, scope.ID, versionID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) DeleteScope(ctx context.Context, id string) error {
	var used bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schedule_plans WHERE execution_scope_id=?)`, id).Scan(&used); err != nil {
		return err
	}
	if used {
		return fmt.Errorf("execution scope %q cannot be deleted because it is used by a schedule plan", id)
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM execution_scopes WHERE id=?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) FindScope(ctx context.Context, id string) (*domain.ExecutionScope, error) {
	var scope domain.ExecutionScope
	var metadata string
	err := r.db.QueryRowContext(ctx, `SELECT id, name, COALESCE(network_topology_id, ''), metadata
		FROM execution_scopes WHERE id=?`, id).Scan(
		&scope.ID, &scope.Name, &scope.NetworkTopologyID, &metadata,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(metadata), &scope.Metadata); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT environment_version_id
		FROM execution_scope_environments WHERE execution_scope_id=?
		ORDER BY environment_version_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var versionID string
		if err := rows.Scan(&versionID); err != nil {
			return nil, err
		}
		scope.EnvironmentVersionIDs = append(scope.EnvironmentVersionIDs, versionID)
	}
	return &scope, rows.Err()
}

func (r *Repository) ListScopes(ctx context.Context) ([]domain.ExecutionScope, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM execution_scopes ORDER BY name`)
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
	scopes := make([]domain.ExecutionScope, 0, len(ids))
	for _, id := range ids {
		scope, err := r.FindScope(ctx, id)
		if err != nil {
			return nil, err
		}
		if scope != nil {
			scopes = append(scopes, *scope)
		}
	}
	return scopes, nil
}
