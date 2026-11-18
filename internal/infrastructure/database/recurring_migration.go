package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database/schema"
)

const beforeExecutionSchedules = "92f25ba449c1f16fb3db785b382940f6dff2116f8aa28dcdc0479f4a85c64730"

func addExecutionSchedules(ctx context.Context, db *sql.DB) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var count int
	var checksum string
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(MAX(checksum), '') FROM schema_metadata").Scan(&count, &checksum); err != nil {
		return false, nil
	}
	if count != 1 || checksum != beforeExecutionSchedules {
		return false, nil
	}
	_, addition, found := strings.Cut(schema.SQL, "-- Recurring execution schedules\n")
	if !found {
		return false, fmt.Errorf("recurring execution schedule schema is missing")
	}
	if _, err := tx.ExecContext(ctx, addition); err != nil {
		return false, fmt.Errorf("add recurring execution schedule schema: %w", err)
	}
	for _, table := range requiredTables() {
		if err := requireTable(ctx, tx, table); err != nil {
			return false, err
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE schema_metadata SET checksum=?, applied_at=?", schemaChecksum(), time.Now().UTC()); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
