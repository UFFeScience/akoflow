package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestOpenCreatesParentDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "database", "akoflow.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database was not created: %v", err)
	}
}

func TestOpenExpandsQuotedWorkspaceVariable(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("AKOFLOW_TEST_WORKSPACE", workspace)
	path := `"$AKOFLOW_TEST_WORKSPACE/storage/kind-demo.db"`
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(workspace, "storage", "kind-demo.db")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expanded database was not created at %q: %v", expected, err)
	}
}

func TestBootstrapInstallsAndValidatesCanonicalSchema(t *testing.T) {
	db := memoryDatabase(t)
	ctx := context.Background()
	if err := Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := Bootstrap(ctx, db); err != nil {
		t.Fatalf("valid schema should be reusable: %v", err)
	}
	var checksum string
	if err := db.QueryRow(`SELECT checksum FROM schema_metadata`).Scan(&checksum); err != nil {
		t.Fatal(err)
	}
	if checksum != schemaChecksum() {
		t.Fatalf("metadata checksum = %q", checksum)
	}
}

func TestBootstrapRejectsPartialDatabase(t *testing.T) {
	db := memoryDatabase(t)
	if _, err := db.Exec(`CREATE TABLE stray(id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	err := Bootstrap(context.Background(), db)
	if !errors.Is(err, ErrIncompatibleSchema) {
		t.Fatalf("expected incompatible schema, got %v", err)
	}
}

func TestBootstrapRejectsChangedMetadata(t *testing.T) {
	db := memoryDatabase(t)
	ctx := context.Background()
	if err := Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE schema_metadata SET checksum='changed'`); err != nil {
		t.Fatal(err)
	}
	if err := Validate(ctx, db); !errors.Is(err, ErrIncompatibleSchema) {
		t.Fatalf("expected incompatible schema, got %v", err)
	}
}

func TestValidateRejectsMissingCanonicalTable(t *testing.T) {
	db := memoryDatabase(t)
	ctx := context.Background()
	if err := Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TABLE discovery_runs`); err != nil {
		t.Fatal(err)
	}
	if err := Validate(ctx, db); !errors.Is(err, ErrIncompatibleSchema) {
		t.Fatalf("expected incompatible schema, got %v", err)
	}
}

func memoryDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}
