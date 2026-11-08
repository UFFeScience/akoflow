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

func TestBootstrapRejectsLegacySchemaInsteadOfMigratingIt(t *testing.T) {
	db := memoryDatabase(t)
	if _, err := db.Exec(`CREATE TABLE schema_metadata (checksum TEXT NOT NULL, applied_at DATETIME NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_metadata(checksum, applied_at) VALUES ('legacy', CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if err := Bootstrap(context.Background(), db); !errors.Is(err, ErrIncompatibleSchema) {
		t.Fatalf("expected incompatible schema, got %v", err)
	}
}

func TestBootstrapRejectsPreviouslyMigratableSchemasWithoutChanges(t *testing.T) {
	for _, checksum := range []string{
		"a961b02a2f2cd9adce4e78b2af1ad3428f1a37540af9890c738ac7a7f6799a17",
		"4d9f0a40c2f6dc40941cacb0fc9ab6f547cd2447cd61ac0c11522434f3b256c8",
		"7babdb5a0b9b80ede464fe6f364d60437cd33b85917651b09acb6b9806955352",
		"2361bba3c44288e038ce3a46b7d32e325b359da71a112607af28ecc6af0e2359",
		"118459cfa1b2ace00d67b0e6f2a29e0f626525fab0a725221c66b6c814a5a313",
	} {
		t.Run(checksum, func(t *testing.T) {
			db := memoryDatabase(t)
			if _, err := db.Exec(`CREATE TABLE schema_metadata (checksum TEXT NOT NULL, applied_at DATETIME NOT NULL);
CREATE TABLE retained (id TEXT PRIMARY KEY);
INSERT INTO retained VALUES ('preserved');`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO schema_metadata VALUES (?, CURRENT_TIMESTAMP)`, checksum); err != nil {
				t.Fatal(err)
			}
			var before int
			if err := db.QueryRow(`PRAGMA schema_version`).Scan(&before); err != nil {
				t.Fatal(err)
			}
			if err := Bootstrap(context.Background(), db); !errors.Is(err, ErrIncompatibleSchema) {
				t.Fatalf("expected incompatible schema, got %v", err)
			}
			var after int
			var stored, retained string
			if err := db.QueryRow(`PRAGMA schema_version`).Scan(&after); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT checksum FROM schema_metadata`).Scan(&stored); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT id FROM retained`).Scan(&retained); err != nil {
				t.Fatal(err)
			}
			if before != after || stored != checksum || retained != "preserved" {
				t.Fatalf("legacy database changed: schema %d -> %d, checksum %s, retained %s", before, after, stored, retained)
			}
		})
	}
}

func TestBootstrapIncludesSchedulerCatalogAndPreservesItOnReopen(t *testing.T) {
	db := memoryDatabase(t)
	ctx := context.Background()
	if err := Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO scheduler_plugin_sources VALUES ('custom', 'draft', 'active');
INSERT INTO scheduler_plugin_revisions VALUES ('custom', 1, 'revision');`); err != nil {
		t.Fatal(err)
	}
	if err := Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	var payload string
	if err := db.QueryRow(`SELECT payload FROM scheduler_plugin_revisions WHERE id='custom' AND revision=1`).Scan(&payload); err != nil || payload != "revision" {
		t.Fatalf("catalog revision changed: %q, %v", payload, err)
	}
}

func TestBootstrapDoesNotModifyIncompatibleDatabase(t *testing.T) {
	db := memoryDatabase(t)
	if _, err := db.Exec(`CREATE TABLE retained (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO retained(id) VALUES ('unchanged')`); err != nil {
		t.Fatal(err)
	}
	if err := Bootstrap(context.Background(), db); !errors.Is(err, ErrIncompatibleSchema) {
		t.Fatalf("expected incompatible schema, got %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM retained WHERE id='unchanged'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("retained rows = %d, want 1", count)
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
