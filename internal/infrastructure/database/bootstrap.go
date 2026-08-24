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
	if err := Validate(ctx, db); err != nil {
		return fmt.Errorf("%w; remove the existing database file and recreate it: %v", ErrIncompatibleSchema, err)
	}
	return nil
}

// legacySchemaBeforeUserPreferences is the only schema version that can be
// upgraded in place. Other checksum mismatches continue to be rejected rather
// than masking a damaged or manually altered database.
const legacySchemaBeforeUserPreferences = "e69b7b1e006cddda2f49939e41e5b2c84b7fddb61317a38216258e28b55ef250"
const schemaBeforeExecutionMetrics = "9bf9465dbc586d92d41480a45fa5d680653ddb6aba0402b132e992fc91c0b9ac"

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
	if _, err := tx.ExecContext(ctx, `UPDATE schema_metadata SET checksum=?, applied_at=?`, schemaChecksum(), time.Now().UTC()); err != nil {
		return fmt.Errorf("record execution metrics migration: %w", err)
	}
	return tx.Commit()
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
