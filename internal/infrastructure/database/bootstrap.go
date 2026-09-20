package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database/schema"
)

// Bootstrap installs the complete canonical schema in a single transaction
// when the database is empty. Existing databases are never patched in place:
// they must already match the canonical schema or be recreated.
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
	if err := applyMigrations(ctx, db); err != nil {
		return err
	}
	if err := Validate(ctx, db); err != nil {
		return fmt.Errorf("%w; recreate the database from the canonical bootstrap: %v", ErrIncompatibleSchema, err)
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
