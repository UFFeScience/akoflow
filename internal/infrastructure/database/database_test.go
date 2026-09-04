package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/infrastructure/database/schema"
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

func TestOpenReadOnlyRejectsWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE example (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	snapshot, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	if _, err := snapshot.Exec(`INSERT INTO example (id) VALUES ('write')`); err == nil {
		t.Fatal("expected a read-only database to reject writes")
	}
}

func TestOpenReadOnlyObservesNewWALCommits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.db")
	writer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if _, err := writer.Exec(`CREATE TABLE example (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	reader, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err := writer.Exec(`INSERT INTO example (id) VALUES ('visible')`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := reader.QueryRow(`SELECT COUNT(*) FROM example`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("read-only connection observed %d rows, want 1", count)
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

func TestBootstrapAddsCloudRuntimeDriverToExistingDatabase(t *testing.T) {
	db := memoryDatabase(t)
	oldSchema := strings.Replace(
		schema.SQL,
		"'serverless', 'simgrid', 'cloud'",
		"'serverless', 'simgrid'",
		1,
	)
	if _, err := db.Exec(oldSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_metadata(checksum, applied_at) VALUES (?, CURRENT_TIMESTAMP)`, schemaBeforeCloudRuntimeDriver); err != nil {
		t.Fatal(err)
	}
	if err := Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO environments(id, name, description, status) VALUES ('env', 'Cloud', '', 'defined')`,
		`INSERT INTO environment_versions(id, environment_id, version, status, network_model, interference_model, cost_model, configuration_hash) VALUES ('env-v1', 'env', 1, 'draft', 'static-links', 'none', 'per-second', 'hash')`,
		`INSERT INTO environment_runtimes(id, environment_version_id, name, driver, mode) VALUES ('cloud-runtime', 'env-v1', 'Cloud', 'cloud', 'execution')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
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
