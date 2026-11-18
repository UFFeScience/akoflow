package database

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database/schema"
)

func TestRecurringMigrationPreservesExistingData(t *testing.T) {
	ctx := context.Background()
	db, err := Open(t.TempDir() + "/previous.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	previous, _, ok := strings.Cut(schema.SQL, "\n-- Recurring execution schedules\n")
	if !ok || fmt.Sprintf("%x", sha256.Sum256([]byte(previous))) != beforeExecutionSchedules {
		t.Fatal("previous schema no longer matches migration baseline")
	}
	if _, err := db.ExecContext(ctx, previous); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO schema_metadata(checksum,applied_at) VALUES(?,CURRENT_TIMESTAMP)", beforeExecutionSchedules); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO environments(id,name) VALUES('existing','Existing environment')"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := Bootstrap(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	var name string
	if err := db.QueryRowContext(ctx, "SELECT name FROM environments WHERE id='existing'").Scan(&name); err != nil || name != "Existing environment" {
		t.Fatalf("existing data was not preserved: %q, %v", name, err)
	}
	if err := Validate(ctx, db); err != nil {
		t.Fatal(err)
	}
}
