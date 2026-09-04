package provenance

import (
	"context"
	"database/sql"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	_ "github.com/mattn/go-sqlite3"
)

func TestQueryUsesWhitelistedEntityAndPagination(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE workflow_definitions (id TEXT, external_id TEXT, name TEXT, namespace TEXT, created_at DATETIME);
		INSERT INTO workflow_definitions VALUES ('wf-1','external-1','Climate workflow','science','2026-01-01');
		INSERT INTO workflow_definitions VALUES ('wf-2','external-2','Genomics workflow','science','2026-01-02');`); err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	page, err := repository.Query(context.Background(), ports.ProvenanceQuery{Entity: "workflows", Search: "climate", Page: 1, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0]["id"] != "wf-1" {
		t.Fatalf("unexpected page: %#v", page)
	}
}

func TestQueryRejectsUnknownEntityAndField(t *testing.T) {
	repository := New(&sql.DB{})
	if _, err := repository.Query(context.Background(), ports.ProvenanceQuery{Entity: "sqlite_master"}); err == nil {
		t.Fatal("unknown entity must be rejected")
	}
	if _, err := repository.Query(context.Background(), ports.ProvenanceQuery{Entity: "workflows", FilterField: "raw_definition"}); err == nil {
		t.Fatal("unknown field must be rejected")
	}
}

func TestSQLAllowsSelectAndRejectsWrites(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE runs (id TEXT, status TEXT); INSERT INTO runs VALUES ('run-1', 'completed')"); err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	result, err := repository.SQL(context.Background(), ports.ProvenanceSQLQuery{
		SQL: "SELECT id, status FROM runs WHERE status = :status", Parameters: map[string]any{"status": "completed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0]["id"] != "run-1" {
		t.Fatalf("unexpected SQL result: %#v", result)
	}
	if _, err := repository.SQL(context.Background(), ports.ProvenanceSQLQuery{SQL: "DELETE FROM runs"}); err == nil {
		t.Fatal("write statement must be rejected")
	}
}

func TestLineageTraversesCatalogRelationships(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE workflow_definitions (id TEXT, external_id TEXT, name TEXT, namespace TEXT, created_at DATETIME);
		CREATE TABLE workflow_versions (id TEXT, workflow_id TEXT, version INTEGER, definition_hash TEXT, status TEXT, created_at DATETIME);
		INSERT INTO workflow_definitions VALUES ('wf-1', 'external', 'Climate', 'science', '2026-01-01');
		INSERT INTO workflow_versions VALUES ('version-1', 'wf-1', 1, 'hash', 'published', '2026-01-01');
	`); err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	result, err := repository.Lineage(context.Background(), ports.ProvenanceLineageQuery{
		Entity: "workflow_versions", ID: "version-1", Direction: "upstream", Depth: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Nodes) != 2 || len(result.Edges) != 1 || result.Root != "workflow_versions:version-1" {
		t.Fatalf("unexpected lineage: %#v", result)
	}
}
