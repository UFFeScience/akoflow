package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const schemaBeforeActivityWorkspaces = "a961b02a2f2cd9adce4e78b2af1ad3428f1a37540af9890c738ac7a7f6799a17"
const schemaWithActivityWorkspaces = "4d9f0a40c2f6dc40941cacb0fc9ab6f547cd2447cd61ac0c11522434f3b256c8"
const schemaWithWorkflowOperationEvents = "7babdb5a0b9b80ede464fe6f364d60437cd33b85917651b09acb6b9806955352"
const schemaWithRunFeedIndexes = "2361bba3c44288e038ce3a46b7d32e325b359da71a112607af28ecc6af0e2359"

func applyMigrations(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(checksum), '') FROM schema_metadata`).Scan(&checksum); err != nil {
		return nil
	}
	for checksum != schemaChecksum() {
		switch checksum {
		case schemaBeforeActivityWorkspaces:
			if err := applyMigration(ctx, db, activityWorkspaceMigrationSQL, schemaWithActivityWorkspaces, "add activity workspace lifecycle schema"); err != nil {
				return err
			}
			checksum = schemaWithActivityWorkspaces
		case schemaWithActivityWorkspaces:
			if err := applyMigration(ctx, db, workflowOperationEventsMigrationSQL, schemaWithWorkflowOperationEvents, "add workflow operation events schema"); err != nil {
				return err
			}
			checksum = schemaWithWorkflowOperationEvents
		case schemaWithWorkflowOperationEvents:
			if err := applyMigration(ctx, db, runFeedIndexesSQL, schemaWithRunFeedIndexes, "index execution feeds and relationships"); err != nil {
				return err
			}
			checksum = schemaWithRunFeedIndexes
		case schemaWithRunFeedIndexes:
			if err := applyMigration(ctx, db, catalogReadIndexesSQL, schemaChecksum(), "index scoped catalog reads"); err != nil {
				return err
			}
			checksum = schemaChecksum()
		default:
			return nil
		}
	}
	return nil
}

const catalogReadIndexesSQL = `
CREATE INDEX cloud_operations_instance_idx ON cloud_operation_runs(instance_id,created_at DESC,id DESC);
CREATE INDEX cloud_operations_environment_idx ON cloud_operation_runs(environment_id,created_at DESC,id DESC);
CREATE INDEX cloud_operations_target_idx ON cloud_operation_runs(capacity_target_id,created_at DESC,id DESC);
CREATE INDEX cloud_operations_execution_idx ON cloud_operation_runs(execution_run_id,created_at DESC,id DESC);
`

const runFeedIndexesSQL = `
CREATE INDEX execution_runs_started_idx ON execution_runs(started_at DESC, id DESC);
CREATE INDEX execution_runs_plan_idx ON execution_runs(schedule_plan_id);
CREATE INDEX data_transfer_observations_run_idx ON data_transfer_observations(execution_run_id);
CREATE INDEX activity_handles_run_idx ON activity_handles(execution_run_id, activity_id);
CREATE INDEX schedule_plans_workflow_idx ON schedule_plans(workflow_version_id);
CREATE INDEX schedule_plan_assignments_plan_idx ON schedule_plan_assignments(schedule_plan_id);
CREATE INDEX activity_definitions_workflow_idx ON activity_definitions(workflow_version_id);
CREATE INDEX console_commands_feed_idx ON console_commands(started_at DESC, id DESC, status);
CREATE INDEX console_sessions_feed_idx ON console_sessions(COALESCE(connected_at, created_at) DESC, id DESC, status);
CREATE INDEX data_object_instances_created_idx ON data_object_instances(created_at DESC, id DESC);
CREATE INDEX data_locations_object_idx ON data_locations(data_object_instance_id);
`

func applyMigration(ctx context.Context, db *sql.DB, statement, checksum, description string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("%s: %w", description, err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, checksum, time.Now().UTC()); err != nil {
		return fmt.Errorf("record %s migration: %w", description, err)
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

const workflowOperationEventsMigrationSQL = `
CREATE TABLE workflow_operation_events (
    id TEXT PRIMARY KEY,
    execution_run_id TEXT NOT NULL REFERENCES execution_runs(id) ON DELETE CASCADE,
    activity_id TEXT NOT NULL DEFAULT '', operation_id TEXT NOT NULL,
    parent_operation_id TEXT NOT NULL DEFAULT '', transfer_run_id TEXT NOT NULL DEFAULT '',
    command_id TEXT NOT NULL DEFAULT '', sequence INTEGER NOT NULL,
    level TEXT NOT NULL CHECK(level IN ('debug','info','warning','error')),
    category TEXT NOT NULL, phase TEXT NOT NULL,
    message TEXT NOT NULL, command_sanitized TEXT NOT NULL DEFAULT '',
    progress_bytes INTEGER NOT NULL DEFAULT 0, total_bytes INTEGER NOT NULL DEFAULT 0,
    throughput_bps REAL NOT NULL DEFAULT 0, exit_code INTEGER,
    stdout_excerpt TEXT NOT NULL DEFAULT '', stderr_excerpt TEXT NOT NULL DEFAULT '',
    metadata TEXT NOT NULL DEFAULT '{}', occurred_at DATETIME NOT NULL,
    UNIQUE(operation_id, sequence)
);
CREATE INDEX workflow_operation_events_run_idx ON workflow_operation_events(execution_run_id, occurred_at, sequence);
CREATE INDEX workflow_operation_events_activity_idx ON workflow_operation_events(execution_run_id, activity_id, occurred_at);
CREATE INDEX workflow_operation_events_operation_idx ON workflow_operation_events(operation_id, sequence);
`
