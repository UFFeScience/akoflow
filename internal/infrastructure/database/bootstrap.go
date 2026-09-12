package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database/schema"
)

// Bootstrap installs the canonical schema only into an empty database. Older
// database files are intentionally unsupported and must be recreated.
func Bootstrap(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("bootstrap requires a database")
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}
	empty, err := isEmpty(ctx, db)
	if err != nil {
		return err
	}
	if empty {
		return installSchema(ctx, db)
	}
	if err := migrateUserPreferences(ctx, db); err != nil {
		return err
	}
	if err := migrateExecutionMetrics(ctx, db); err != nil {
		return err
	}
	if err := migrateTransferSettings(ctx, db); err != nil {
		return err
	}
	if err := migrateWorkflowDataDependencies(ctx, db); err != nil {
		return err
	}
	if err := migratePlanningSessions(ctx, db); err != nil {
		return err
	}
	if err := migrateCloudFoundation(ctx, db); err != nil {
		return err
	}
	if err := migrateCloudInstances(ctx, db); err != nil {
		return err
	}
	if err := migrateCloudRuntimeDriver(ctx, db); err != nil {
		return err
	}
	if err := migrateCloudExecutionTarget(ctx, db); err != nil {
		return err
	}
	if err := migrateCloudCatalogSnapshots(ctx, db); err != nil {
		return err
	}
	if err := migrateCloudOperationRuns(ctx, db); err != nil {
		return err
	}
	if err := migrateCloudExecutionDataPlane(ctx, db); err != nil {
		return err
	}
	if err := migrateTransferRunOwnership(ctx, db); err != nil {
		return err
	}
	if err := migrateWorkflowExpansions(ctx, db); err != nil {
		return err
	}
	if err := Validate(ctx, db); err != nil {
		return fmt.Errorf("%w; remove the existing database file and recreate it: %v", ErrIncompatibleSchema, err)
	}
	return cleanupLegacyCloudEntrypoints(ctx, db)
}

// legacySchemaBeforeUserPreferences is the only schema version that can be
// upgraded in place. Other checksum mismatches continue to be rejected rather
// than masking a damaged or manually altered database.
const legacySchemaBeforeUserPreferences = "e69b7b1e006cddda2f49939e41e5b2c84b7fddb61317a38216258e28b55ef250"
const schemaBeforeExecutionMetrics = "9bf9465dbc586d92d41480a45fa5d680653ddb6aba0402b132e992fc91c0b9ac"
const schemaBeforeTransferSettings = "2412be2fc4530cf52b1e620dc2454ab97207980f6be17876b7ad4c357abe5223"
const schemaBeforeWorkflowDataDependencies = "2a07d9d4c5230f9c5cf884142c092eb7bda7b726fdd03b55e7e3174251e80288"
const schemaBeforePlanningSessions = "8f6ed6fec292490f85e3fe8e6bf7edca918375589516a687d8a00d61f882c137"
const schemaBeforeCloudFoundation = "7e1bbb05c5eb78bdaf86806a2a7c18a474b28035ebffee5ce8e0f65b5fffd722"
const schemaBeforeCloudInstances = "5c0308e9166ed23dc481b04192cd1539b96ba60b0543df10c7fb2cd065e8b4bc"
const schemaBeforeCloudRuntimeDriver = "2d4a31031d61e8dd5a8c80550cc47d8fa158d8695cffb979f490342bb8d340a6"
const schemaBeforeCloudExecutionTarget = "e515cbbfe701482f1c952431134d34359284f42959ace947f55faabad269d739"
const schemaBeforeCloudCatalogSnapshots = "c9f8b4a4d2d2fd5bfecc4f289d0e68657f608d3b7a75ba53ee4e946dcf251f0b"
const schemaBeforeCloudOperationRuns = "f96a82d2abb3977da8df5907bec5a3b1b09e5a852d385d1e3daa6c019d5ab75c"
const schemaBeforeCloudExecutionDataPlane = "45527a1e824b489f357e154689780cc164bc6cbbdd46905eeaec774303e5eed1"
const schemaBeforeTransferRunOwnership = "e4e6a07b937bdef39d2056c2606db45762576553351c623e1d9fccd1b2b1de16"
const schemaBeforeWorkflowExpansions = "3827285aac2f21a0af43632c65af5dff749900b45e1f0d06d707f145a031386f"

func migrateWorkflowExpansions(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil || checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforeWorkflowExpansions {
		return nil
	}
	var expansionTable int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='workflow_expansions'`).Scan(&expansionTable); err != nil {
		return fmt.Errorf("inspect workflow expansion schema: %w", err)
	}
	if expansionTable > 0 {
		if _, err := db.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaChecksum(), time.Now().UTC()); err != nil {
			return fmt.Errorf("record workflow expansion migration: %w", err)
		}
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workflow expansion migration: %w", err)
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`CREATE TABLE workflow_expansions (
			id TEXT PRIMARY KEY, workflow_version_id TEXT NOT NULL REFERENCES workflow_versions(id),
			execution_run_id TEXT NOT NULL DEFAULT '', source_activity_id TEXT NOT NULL, source_event_id TEXT NOT NULL,
			sequence INTEGER NOT NULL CHECK(sequence > 0), result_revision INTEGER NOT NULL CHECK(result_revision > 0),
			status TEXT NOT NULL CHECK(status IN ('applied', 'rejected')), failure_reason TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '{}', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(workflow_version_id, source_event_id))`,
		`CREATE UNIQUE INDEX workflow_expansions_applied_sequence_idx
			ON workflow_expansions(workflow_version_id, execution_run_id, sequence) WHERE status='applied'`,
		`CREATE TABLE workflow_expansion_activities (
			expansion_id TEXT NOT NULL REFERENCES workflow_expansions(id), activity_id TEXT NOT NULL,
			definition TEXT NOT NULL, PRIMARY KEY(expansion_id, activity_id))`,
		`CREATE TABLE workflow_expansion_dependencies (
			expansion_id TEXT NOT NULL REFERENCES workflow_expansions(id), activity_id TEXT NOT NULL,
			depends_on_activity_id TEXT NOT NULL, dependency_type TEXT NOT NULL DEFAULT 'control',
			PRIMARY KEY(expansion_id, activity_id, depends_on_activity_id))`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply workflow expansion migration: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaChecksum(), time.Now().UTC()); err != nil {
		return fmt.Errorf("record workflow expansion migration: %w", err)
	}
	return tx.Commit()
}

func migrateUserPreferences(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil {
		return nil
	}
	if checksum == schemaChecksum() {
		return nil
	}
	if checksum != legacySchemaBeforeUserPreferences {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user preferences migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS user_preferences (
		client_id TEXT PRIMARY KEY,
		theme TEXT NOT NULL DEFAULT 'light' CHECK(theme IN ('light', 'dark')),
		animations_enabled INTEGER NOT NULL DEFAULT 1 CHECK(animations_enabled IN (0, 1)),
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create user preferences table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforeExecutionMetrics, time.Now().UTC()); err != nil {
		return fmt.Errorf("record user preferences migration: %w", err)
	}
	return tx.Commit()
}

func migrateExecutionMetrics(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil || checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforeExecutionMetrics {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin execution metrics migration: %w", err)
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`ALTER TABLE transfer_runs ADD COLUMN started_at REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE transfer_runs ADD COLUMN finished_at REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE transfer_runs ADD COLUMN transferred_bytes INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE task_executions ADD COLUMN transfer_bytes INTEGER NOT NULL DEFAULT 0`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply execution metrics migration: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforeTransferSettings, time.Now().UTC()); err != nil {
		return fmt.Errorf("record execution metrics migration: %w", err)
	}
	return tx.Commit()
}

func migrateTransferSettings(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil || checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforeTransferSettings {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transfer settings migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `ALTER TABLE system_instance ADD COLUMN transfer_buffer_bytes INTEGER NOT NULL DEFAULT 8388608`); err != nil {
		return fmt.Errorf("add transfer buffer setting: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforeWorkflowDataDependencies, time.Now().UTC()); err != nil {
		return fmt.Errorf("record transfer settings migration: %w", err)
	}
	return tx.Commit()
}

func migrateWorkflowDataDependencies(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil || checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforeWorkflowDataDependencies {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workflow data dependencies migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE workflow_data_dependencies (
		producer_activity_id TEXT NOT NULL REFERENCES activity_definitions(id),
		consumer_activity_id TEXT NOT NULL REFERENCES activity_definitions(id),
		logical_name TEXT NOT NULL,
		size_bytes INTEGER NOT NULL CHECK(size_bytes > 0),
		PRIMARY KEY(producer_activity_id, consumer_activity_id, logical_name),
		CHECK(producer_activity_id <> consumer_activity_id)
	)`); err != nil {
		return fmt.Errorf("create workflow data dependencies table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforePlanningSessions, time.Now().UTC()); err != nil {
		return fmt.Errorf("record workflow data dependencies migration: %w", err)
	}
	return tx.Commit()
}

func migratePlanningSessions(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil || checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforePlanningSessions {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin planning sessions migration: %w", err)
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE planning_sessions (
			id TEXT PRIMARY KEY, workflow_version_id TEXT NOT NULL REFERENCES workflow_versions(id),
			execution_scope_id TEXT NOT NULL REFERENCES execution_scopes(id),
			network_topology_id TEXT NOT NULL REFERENCES network_topologies(id),
			status TEXT NOT NULL CHECK(status IN ('queued','running','completed','failed','cancelled')),
			algorithms TEXT NOT NULL DEFAULT '[]', progress REAL NOT NULL DEFAULT 0,
			candidate_count INTEGER NOT NULL DEFAULT 0, selected_candidate_id TEXT,
			selected_plan_id TEXT REFERENCES schedule_plans(id), deadline_seconds REAL NOT NULL DEFAULT 0,
			budget REAL NOT NULL DEFAULT 0, configuration TEXT NOT NULL DEFAULT '{}',
			failure_reason TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME, completed_at DATETIME)`,
		`CREATE TABLE planning_algorithm_runs (
			id TEXT PRIMARY KEY, planning_session_id TEXT NOT NULL REFERENCES planning_sessions(id) ON DELETE CASCADE,
			algorithm TEXT NOT NULL, objective TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL CHECK(status IN ('queued','running','completed','failed','cancelled')),
			progress REAL NOT NULL DEFAULT 0, candidate_count INTEGER NOT NULL DEFAULT 0,
			configuration TEXT NOT NULL DEFAULT '{}', failure_reason TEXT NOT NULL DEFAULT '',
			started_at DATETIME, completed_at DATETIME, UNIQUE(planning_session_id, algorithm))`,
		`CREATE TABLE planning_candidates (
			id TEXT PRIMARY KEY, planning_session_id TEXT NOT NULL REFERENCES planning_sessions(id) ON DELETE CASCADE,
			algorithm_run_id TEXT NOT NULL REFERENCES planning_algorithm_runs(id) ON DELETE CASCADE,
			algorithm TEXT NOT NULL, objective TEXT NOT NULL DEFAULT '', rank INTEGER NOT NULL DEFAULT 0,
			pareto_optimal INTEGER NOT NULL DEFAULT 0, dominated INTEGER NOT NULL DEFAULT 0,
			feasible INTEGER NOT NULL DEFAULT 0, predicted_makespan_seconds REAL NOT NULL DEFAULT 0,
			predicted_cost REAL NOT NULL DEFAULT 0, plan TEXT NOT NULL, fingerprint TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(planning_session_id, fingerprint))`,
		`CREATE INDEX idx_planning_candidates_session_rank ON planning_candidates(planning_session_id, rank)`,
		`CREATE INDEX idx_planning_candidates_session_pareto ON planning_candidates(planning_session_id, pareto_optimal, dominated)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply planning sessions migration: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforeCloudFoundation, time.Now().UTC()); err != nil {
		return fmt.Errorf("record planning sessions migration: %w", err)
	}
	return tx.Commit()
}

func migrateCloudFoundation(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil || checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforeCloudFoundation {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cloud foundation migration: %w", err)
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE machine_configurations (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
			ownership TEXT NOT NULL DEFAULT 'user' CHECK(ownership IN ('system','user')),
			enabled INTEGER NOT NULL DEFAULT 1, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE machine_configuration_versions (
			id TEXT PRIMARY KEY, machine_configuration_id TEXT NOT NULL
				REFERENCES machine_configurations(id) ON DELETE CASCADE,
			version INTEGER NOT NULL CHECK(version > 0),
			status TEXT NOT NULL CHECK(status IN ('draft','published','deprecated')),
			playbook_yaml TEXT NOT NULL, content_sha256 TEXT NOT NULL,
			compatibility TEXT NOT NULL DEFAULT '{}', variables_schema TEXT NOT NULL DEFAULT '{}',
			validation_checks TEXT NOT NULL DEFAULT '[]', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(machine_configuration_id,version))`,
		`CREATE TABLE cloud_capacity_targets (
			id TEXT PRIMARY KEY, environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
			name TEXT NOT NULL, provider TEXT NOT NULL CHECK(provider IN ('gcp','aws','azure')),
			provider_machine_type TEXT NOT NULL, region TEXT NOT NULL,
			zone_policy TEXT NOT NULL DEFAULT 'any-compatible', fixed_zone TEXT NOT NULL DEFAULT '',
			image_reference TEXT NOT NULL, architecture TEXT NOT NULL DEFAULT 'amd64',
			vcpu INTEGER NOT NULL CHECK(vcpu > 0), memory_mib INTEGER NOT NULL CHECK(memory_mib > 0),
			provisioning_mode TEXT NOT NULL DEFAULT 'standard'
				CHECK(provisioning_mode IN ('standard','spot')),
			maximum_instances INTEGER NOT NULL DEFAULT 1 CHECK(maximum_instances > 0),
			lifecycle_policy TEXT NOT NULL DEFAULT 'destroy-after-execution',
			configuration TEXT NOT NULL DEFAULT '{}', enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, UNIQUE(environment_id,name))`,
		`CREATE TABLE cloud_capacity_target_configurations (
			capacity_target_id TEXT NOT NULL REFERENCES cloud_capacity_targets(id) ON DELETE CASCADE,
			configuration_version_id TEXT NOT NULL REFERENCES machine_configuration_versions(id),
			execution_order INTEGER NOT NULL CHECK(execution_order >= 0), variables TEXT NOT NULL DEFAULT '{}',
			required INTEGER NOT NULL DEFAULT 0, enabled INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY(capacity_target_id,configuration_version_id),
			UNIQUE(capacity_target_id,execution_order))`,
		`CREATE INDEX cloud_capacity_targets_environment_idx ON cloud_capacity_targets(environment_id,enabled)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply cloud foundation migration: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforeCloudInstances, time.Now().UTC()); err != nil {
		return fmt.Errorf("record cloud foundation migration: %w", err)
	}
	return tx.Commit()
}

func migrateCloudInstances(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil || checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforeCloudInstances {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cloud instances migration: %w", err)
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE cloud_provisioned_instances (
			id TEXT PRIMARY KEY, capacity_target_id TEXT NOT NULL REFERENCES cloud_capacity_targets(id),
			environment_id TEXT NOT NULL REFERENCES environments(id), provider TEXT NOT NULL,
			provider_id TEXT NOT NULL DEFAULT '', name TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('provisioning','configuring','ready','stopped','destroying','destroyed','failed')),
			public_address TEXT NOT NULL DEFAULT '', private_address TEXT NOT NULL DEFAULT '',
			ssh_username TEXT NOT NULL DEFAULT '', ssh_credential_ref TEXT NOT NULL DEFAULT '',
			disk TEXT NOT NULL DEFAULT '{}', terraform_output TEXT NOT NULL DEFAULT '{}',
			failure_reason TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL,
			ready_at DATETIME, destroyed_at DATETIME)`,
		`CREATE INDEX cloud_instances_environment_status_idx ON cloud_provisioned_instances(environment_id,status)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply cloud instances migration: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforeCloudRuntimeDriver, time.Now().UTC()); err != nil {
		return fmt.Errorf("record cloud instances migration: %w", err)
	}
	return tx.Commit()
}

func migrateCloudRuntimeDriver(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil || checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforeCloudRuntimeDriver {
		return nil
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("disable foreign keys for cloud runtime migration: %w", err)
	}
	defer db.ExecContext(context.Background(), `PRAGMA foreign_keys = ON`)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cloud runtime migration: %w", err)
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE environment_runtimes_new (
			id TEXT PRIMARY KEY,
			environment_version_id TEXT NOT NULL REFERENCES environment_versions(id),
			name TEXT NOT NULL,
			driver TEXT NOT NULL CHECK(driver IN ('slurm', 'kubernetes', 'ssh', 'local', 'serverless', 'simgrid', 'cloud')),
			mode TEXT NOT NULL CHECK(mode IN ('execution', 'simulation')),
			role TEXT NOT NULL DEFAULT '',
			configuration TEXT NOT NULL DEFAULT '{}',
			UNIQUE(environment_version_id, name),
			UNIQUE(id, environment_version_id)
		)`,
		`INSERT INTO environment_runtimes_new
			(id, environment_version_id, name, driver, mode, role, configuration)
		 SELECT id, environment_version_id, name, driver, mode, role, configuration
		 FROM environment_runtimes`,
		`DROP TABLE environment_runtimes`,
		`ALTER TABLE environment_runtimes_new RENAME TO environment_runtimes`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply cloud runtime migration: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforeCloudExecutionTarget, time.Now().UTC()); err != nil {
		return fmt.Errorf("record cloud runtime migration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cloud runtime migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("restore foreign keys after cloud runtime migration: %w", err)
	}
	return nil
}

func migrateCloudCatalogSnapshots(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil || checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforeCloudCatalogSnapshots {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cloud catalog migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS cloud_catalog_snapshots (
		environment_id TEXT PRIMARY KEY REFERENCES environments(id) ON DELETE CASCADE,
		catalog TEXT NOT NULL,
		discovered_at DATETIME NOT NULL
	)`); err != nil {
		return fmt.Errorf("apply cloud catalog migration: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforeCloudOperationRuns, time.Now().UTC()); err != nil {
		return fmt.Errorf("record cloud catalog migration: %w", err)
	}
	return tx.Commit()
}

func migrateCloudOperationRuns(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil || checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforeCloudOperationRuns {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cloud operation runs migration: %w", err)
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE IF NOT EXISTS cloud_operation_runs (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL CHECK(kind IN ('provision','configure','destroy')),
			status TEXT NOT NULL CHECK(status IN ('queued','running','completed','failed')),
			environment_id TEXT NOT NULL REFERENCES environments(id),
			capacity_target_id TEXT NOT NULL DEFAULT '', instance_id TEXT NOT NULL DEFAULT '',
			request TEXT NOT NULL DEFAULT '{}', failure_reason TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL, started_at DATETIME, finished_at DATETIME)`,
		`CREATE INDEX IF NOT EXISTS cloud_operation_runs_created_idx ON cloud_operation_runs(created_at DESC)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply cloud operation runs migration: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforeCloudExecutionDataPlane, time.Now().UTC()); err != nil {
		return fmt.Errorf("record cloud operation runs migration: %w", err)
	}
	return tx.Commit()
}

func migrateCloudExecutionDataPlane(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil || checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforeCloudExecutionDataPlane {
		return nil
	}
	// Tests and development builds can contain the new physical schema while
	// still carrying the previous migration checksum. In that case there is
	// nothing to rebuild; only advance the migration marker.
	var executionRunColumn int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('cloud_operation_runs') WHERE name='execution_run_id'`).Scan(&executionRunColumn); err != nil {
		return fmt.Errorf("inspect cloud operation schema: %w", err)
	}
	if executionRunColumn > 0 {
		if _, err := db.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforeWorkflowExpansions, time.Now().UTC()); err != nil {
			return fmt.Errorf("record cloud execution data plane migration: %w", err)
		}
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cloud execution data plane migration: %w", err)
	}
	defer tx.Rollback()
	statements := []string{
		`ALTER TABLE cloud_operation_runs RENAME TO cloud_operation_runs_legacy`,
		`CREATE TABLE cloud_operation_runs (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL CHECK(kind IN ('provision','configure','validate','start','stop','destroy','attach-volume','detach-volume')),
			status TEXT NOT NULL CHECK(status IN ('queued','running','completed','failed','cancelled')),
			environment_id TEXT NOT NULL REFERENCES environments(id), capacity_target_id TEXT NOT NULL DEFAULT '',
			instance_id TEXT NOT NULL DEFAULT '', execution_run_id TEXT NOT NULL DEFAULT '', activity_id TEXT NOT NULL DEFAULT '',
			phase TEXT NOT NULL DEFAULT 'queued', request TEXT NOT NULL DEFAULT '{}', failure_reason TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL, started_at DATETIME, finished_at DATETIME)`,
		`INSERT INTO cloud_operation_runs(id,kind,status,environment_id,capacity_target_id,instance_id,request,failure_reason,created_at,started_at,finished_at)
			SELECT id,kind,status,environment_id,capacity_target_id,instance_id,request,failure_reason,created_at,started_at,finished_at FROM cloud_operation_runs_legacy`,
		`DROP TABLE cloud_operation_runs_legacy`,
		`CREATE INDEX cloud_operation_runs_created_idx ON cloud_operation_runs(created_at DESC)`,
		`CREATE TABLE cloud_operation_events (
			operation_id TEXT NOT NULL REFERENCES cloud_operation_runs(id) ON DELETE CASCADE, sequence INTEGER NOT NULL,
			timestamp DATETIME NOT NULL, tool TEXT NOT NULL DEFAULT '', phase TEXT NOT NULL DEFAULT '', level TEXT NOT NULL DEFAULT 'info',
			event TEXT NOT NULL, task TEXT NOT NULL DEFAULT '', host TEXT NOT NULL DEFAULT '', message TEXT NOT NULL DEFAULT '',
			raw TEXT NOT NULL DEFAULT '', duration_seconds REAL NOT NULL DEFAULT 0, PRIMARY KEY(operation_id, sequence))`,
		`ALTER TABLE transfer_runs ADD COLUMN logical_bytes INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE transfer_runs ADD COLUMN network_bytes INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE transfer_runs ADD COLUMN route TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE transfer_runs ADD COLUMN execution_run_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE transfer_runs ADD COLUMN activity_id TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE transfer_chunk_runs (
			transfer_run_id TEXT NOT NULL REFERENCES transfer_runs(id) ON DELETE CASCADE, chunk_index INTEGER NOT NULL,
			offset_bytes INTEGER NOT NULL, size_bytes INTEGER NOT NULL, digest TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL CHECK(status IN ('planned','running','completed','failed')), attempts INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(transfer_run_id, chunk_index))`,
		`CREATE TABLE planned_lifecycle_actions (
			id TEXT PRIMARY KEY, schedule_plan_id TEXT NOT NULL REFERENCES schedule_plans(id) ON DELETE CASCADE,
			capacity_target_id TEXT NOT NULL, cloud_instance_id TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL CHECK(action IN ('provision','configure','validate','start','stop','destroy','attach-volume','detach-volume')),
			earliest_start REAL NOT NULL DEFAULT 0, expected_duration REAL NOT NULL DEFAULT 0,
			depends_on TEXT NOT NULL DEFAULT '[]', metadata TEXT NOT NULL DEFAULT '{}')`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply cloud execution data plane migration: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforeWorkflowExpansions, time.Now().UTC()); err != nil {
		return fmt.Errorf("record cloud execution data plane migration: %w", err)
	}
	return tx.Commit()
}

func migrateTransferRunOwnership(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil || checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforeTransferRunOwnership {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transfer run ownership migration: %w", err)
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`ALTER TABLE transfer_runs ADD COLUMN execution_run_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE transfer_runs ADD COLUMN activity_id TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply transfer run ownership migration: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforeWorkflowExpansions, time.Now().UTC()); err != nil {
		return fmt.Errorf("record transfer run ownership migration: %w", err)
	}
	return tx.Commit()
}

func cleanupLegacyCloudEntrypoints(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `DELETE FROM resources
		WHERE type='cloud_vm'
		AND execution_target='provisioned'
		AND schedulable=0
		AND provider_id='local-engine'
		AND metadata IN ('{}', 'null')
		AND region=''
		AND zone=''
		AND EXISTS (
			SELECT 1 FROM environment_versions version
			JOIN environment_connections connection ON connection.environment_id=version.environment_id
			WHERE version.id=resources.environment_version_id AND connection.type='cloud'
		)
		AND NOT EXISTS (SELECT 1 FROM cloud_capacity_targets target WHERE target.id=resources.id)`)
	if err != nil {
		return fmt.Errorf("remove legacy cloud entrypoint resources: %w", err)
	}
	return nil
}

func migrateCloudExecutionTarget(ctx context.Context, db *sql.DB) error {
	var checksum string
	if err := db.QueryRowContext(ctx, `SELECT checksum FROM schema_metadata LIMIT 1`).Scan(&checksum); err != nil || checksum == schemaChecksum() {
		return nil
	}
	if checksum != schemaBeforeCloudExecutionTarget {
		return nil
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("disable foreign keys for cloud resource migration: %w", err)
	}
	defer db.ExecContext(context.Background(), `PRAGMA foreign_keys = ON`)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cloud resource migration: %w", err)
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE resources_new (
			id TEXT PRIMARY KEY,
			environment_version_id TEXT NOT NULL REFERENCES environment_versions(id),
			execution_target TEXT NOT NULL DEFAULT 'batch'
				CHECK(execution_target IN ('batch', 'direct', 'provisioned')),
			parent_resource_id TEXT REFERENCES resources_new(id),
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			tier TEXT NOT NULL DEFAULT '',
			region TEXT NOT NULL DEFAULT '',
			zone TEXT NOT NULL DEFAULT '',
			architecture TEXT NOT NULL DEFAULT '',
			cpu_cores INTEGER NOT NULL DEFAULT 0,
			cpu_capacity REAL NOT NULL DEFAULT 0,
			memory_bytes INTEGER NOT NULL DEFAULT 0,
			storage_bytes INTEGER NOT NULL DEFAULT 0,
			compute_speedup REAL NOT NULL DEFAULT 1,
			price_per_second REAL NOT NULL DEFAULT 0,
			boot_overhead_seconds REAL NOT NULL DEFAULT 0,
			container_overhead_seconds REAL NOT NULL DEFAULT 0,
			schedulable INTEGER NOT NULL DEFAULT 1,
			metadata TEXT NOT NULL DEFAULT '{}',
			UNIQUE(environment_version_id, provider_id)
		)`,
		`INSERT INTO resources_new (
			id, environment_version_id, execution_target, parent_resource_id, type, name,
			provider_id, tier, region, zone, architecture, cpu_cores, cpu_capacity,
			memory_bytes, storage_bytes, compute_speedup, price_per_second,
			boot_overhead_seconds, container_overhead_seconds, schedulable, metadata)
		 SELECT id, environment_version_id, execution_target, parent_resource_id, type, name,
			provider_id, tier, region, zone, architecture, cpu_cores, cpu_capacity,
			memory_bytes, storage_bytes, compute_speedup, price_per_second,
			boot_overhead_seconds, container_overhead_seconds, schedulable, metadata
		 FROM resources`,
		`DROP TABLE resources`,
		`ALTER TABLE resources_new RENAME TO resources`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply cloud resource migration: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaBeforeCloudCatalogSnapshots, time.Now().UTC()); err != nil {
		return fmt.Errorf("record cloud resource migration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cloud resource migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("restore foreign keys after cloud resource migration: %w", err)
	}
	return nil
}

func installSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin schema bootstrap: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, schema.SQL); err != nil {
		return fmt.Errorf("apply canonical schema: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_metadata(checksum, applied_at) VALUES (?, ?)`, schemaChecksum(), time.Now().UTC()); err != nil {
		return fmt.Errorf("record schema metadata: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema bootstrap: %w", err)
	}
	return Validate(ctx, db)
}

func isEmpty(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("inspect database schema: %w", err)
	}
	return count == 0, nil
}

func schemaChecksum() string { return fmt.Sprintf("%x", sha256.Sum256([]byte(schema.SQL))) }
