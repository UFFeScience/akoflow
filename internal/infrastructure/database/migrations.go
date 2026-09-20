package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const schemaBeforeActivityWorkspaces = "a961b02a2f2cd9adce4e78b2af1ad3428f1a37540af9890c738ac7a7f6799a17"

func applyMigrations(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(checksum), '') FROM schema_metadata`).Scan(&checksum); err != nil {
		return nil
	}
	if checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforeActivityWorkspaces {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, activityWorkspaceMigrationSQL); err != nil {
		return fmt.Errorf("add activity workspace lifecycle schema: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaChecksum(), time.Now().UTC()); err != nil {
		return fmt.Errorf("record activity workspace schema migration: %w", err)
	}
	return tx.Commit()
}

const activityWorkspaceMigrationSQL = `
CREATE TABLE activity_workspaces (
    id TEXT PRIMARY KEY,
    execution_run_id TEXT NOT NULL REFERENCES execution_runs(id) ON DELETE CASCADE,
    activity_id TEXT NOT NULL,
    environment_id TEXT NOT NULL DEFAULT '', resource_id TEXT NOT NULL,
    runtime_id TEXT NOT NULL, connection_id TEXT NOT NULL DEFAULT '', uri TEXT NOT NULL,
    execution_path TEXT NOT NULL, observation_path TEXT NOT NULL, driver TEXT NOT NULL,
    state TEXT NOT NULL CHECK(state IN ('planned','active','sealed','releasable','releasing','released','failed')),
    retention TEXT NOT NULL CHECK(retention IN ('intermediate','final','pinned')),
    is_final INTEGER NOT NULL DEFAULT 0, pinned INTEGER NOT NULL DEFAULT 0,
    manifest TEXT NOT NULL DEFAULT '{}', file_count INTEGER NOT NULL DEFAULT 0,
    size_bytes INTEGER NOT NULL DEFAULT 0, input_bytes INTEGER NOT NULL DEFAULT 0,
    output_bytes INTEGER NOT NULL DEFAULT 0, reclaimed_bytes INTEGER NOT NULL DEFAULT 0,
    release_reason TEXT NOT NULL DEFAULT '', last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL, sealed_at DATETIME, released_at DATETIME,
    UNIQUE(execution_run_id, activity_id)
);
CREATE INDEX activity_workspaces_run_idx ON activity_workspaces(execution_run_id, activity_id);
CREATE INDEX activity_workspaces_state_idx ON activity_workspaces(state, created_at);
CREATE TABLE workspace_leases (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES activity_workspaces(id) ON DELETE CASCADE,
    execution_run_id TEXT NOT NULL REFERENCES execution_runs(id) ON DELETE CASCADE,
    producer_activity_id TEXT NOT NULL, consumer_activity_id TEXT NOT NULL,
    released INTEGER NOT NULL DEFAULT 0, released_at DATETIME,
    release_reason TEXT NOT NULL DEFAULT '',
    UNIQUE(execution_run_id, producer_activity_id, consumer_activity_id)
);
CREATE INDEX workspace_leases_workspace_idx ON workspace_leases(workspace_id, released);
`
