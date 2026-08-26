package database

import (
	"context"
	"database/sql"
	"fmt"
)

// Reset removes persisted user data while preserving the schema and immutable
// system instance identity required for the daemon to remain reachable.
func Reset(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT IN ('schema_metadata', 'system_instance', 'sqlite_sequence')`)
	if err != nil { return fmt.Errorf("list database tables: %w", err) }
	defer rows.Close()
	var names []string
	for rows.Next() { var name string; if err := rows.Scan(&name); err != nil { return err }; names = append(names, name) }
	if err := rows.Err(); err != nil { return err }
	tx, err := db.BeginTx(ctx, nil); if err != nil { return err }
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil { return err }
	for _, name := range names {
		if _, err = tx.ExecContext(ctx, `DELETE FROM "`+name+`"`); err != nil { return fmt.Errorf("clear %s: %w", name, err) }
	}
	return tx.Commit()
}
