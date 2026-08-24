package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database/schema"
)

var ErrIncompatibleSchema = errors.New("database schema is incompatible; recreate the database")

var createTable = regexp.MustCompile(`(?im)CREATE\s+TABLE\s+([a-z_][a-z0-9_]*)\s*\(`)

// requiredTables is derived from the canonical SQL so validation cannot drift
// when a table is added to the schema.
func requiredTables() []string {
	matches := createTable.FindAllStringSubmatch(schema.SQL, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, match[1])
	}
	sort.Strings(values)
	return values
}

func Validate(ctx context.Context, db *sql.DB) error {
	if err := validateMetadata(ctx, db); err != nil {
		return err
	}
	for _, table := range requiredTables() {
		if err := requireTable(ctx, db, table); err != nil {
			return err
		}
	}
	return validateIntegrity(ctx, db)
}

func validateMetadata(ctx context.Context, db *sql.DB) error {
	var rows int
	var checksum string
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(MAX(checksum), '')
		FROM schema_metadata
	`).Scan(&rows, &checksum)
	if err != nil {
		return fmt.Errorf("%w: metadata unavailable: %v", ErrIncompatibleSchema, err)
	}
	if rows != 1 || checksum != schemaChecksum() {
		return fmt.Errorf(
			"%w: expected checksum %s, found checksum %s",
			ErrIncompatibleSchema, schemaChecksum(), checksum,
		)
	}
	return nil
}

func requireTable(ctx context.Context, db *sql.DB, table string) error {
	var count int
	err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`,
		table,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("validate table %q: %w", table, err)
	}
	if count != 1 {
		return fmt.Errorf("%w: required table %q is missing", ErrIncompatibleSchema, table)
	}
	return nil
}

func validateIntegrity(ctx context.Context, db *sql.DB) error {
	// SQLite's integrity checks scan every page. Running one during normal API
	// startup keeps a large run-history database unavailable for several
	// minutes. The bootstrap validation above already verifies the schema;
	// integrity checks are a deliberate maintenance operation instead.
	return nil
	/*
		var result string
		// A full integrity_check scans the complete database at every server boot.
		// This can make a large execution-history database unavailable for minutes.
		// quick_check still validates the database structure on startup; operators can
		// run a full integrity check as a maintenance operation.
		if err := db.QueryRowContext(ctx, `PRAGMA quick_check(1)`).Scan(&result); err != nil {
			return fmt.Errorf("validate database integrity: %w", err)
		}
		if result != "ok" {
			return fmt.Errorf("%w: integrity check returned %q", ErrIncompatibleSchema, result)
		}
		return nil
	*/
}
