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
	if _, err := db.Exec("CREATE TABLE execution_runs (id TEXT, status TEXT); INSERT INTO execution_runs VALUES ('run-1', 'completed')"); err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	result, err := repository.SQL(context.Background(), ports.ProvenanceSQLQuery{
		SQL: "SELECT id, status FROM execution_runs WHERE status = :status", Parameters: map[string]any{"status": "completed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0]["id"] != "run-1" {
		t.Fatalf("unexpected SQL result: %#v", result)
	}
	if _, err := repository.SQL(context.Background(), ports.ProvenanceSQLQuery{SQL: "DELETE FROM execution_runs"}); err == nil {
		t.Fatal("write statement must be rejected")
	}
	if _, err := repository.SQL(context.Background(), ports.ProvenanceSQLQuery{SQL: "SELECT name FROM sqlite_master"}); err == nil {
		t.Fatal("non-allowlisted table must be rejected")
	}
}

func TestSchemaRemainsAvailableAfterReadOnlySQL(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE activity_definitions (id TEXT, workflow_version_id TEXT); INSERT INTO activity_definitions VALUES ('activity-1', 'version-1')`); err != nil {
		t.Fatal(err)
	}
	repository := New(db)
	if _, err := repository.SQL(context.Background(), ports.ProvenanceSQLQuery{SQL: "SELECT id FROM activity_definitions"}); err != nil {
		t.Fatalf("execute read-only SQL: %v", err)
	}
	tables, err := repository.Schema(context.Background())
	if err != nil {
		t.Fatalf("discover schema after SQL: %v", err)
	}
	for _, table := range tables {
		if table.Name == "activity_definitions" {
			return
		}
	}
	t.Fatalf("activity_definitions missing from schema: %#v", tables)
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

func TestLineageIncludesControlAndDataDependencies(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE activity_definitions (id TEXT, workflow_version_id TEXT, external_id TEXT, name TEXT, kind TEXT, priority INTEGER);
		CREATE TABLE activity_dependencies (activity_id TEXT, depends_on_activity_id TEXT, dependency_type TEXT);
		CREATE TABLE workflow_data_dependencies (producer_activity_id TEXT, consumer_activity_id TEXT, logical_name TEXT, size_bytes INTEGER);
		INSERT INTO activity_definitions VALUES ('a', 'version', 'a', 'Producer', 'task', 0), ('b', 'version', 'b', 'Consumer', 'task', 0);
		INSERT INTO activity_dependencies VALUES ('b', 'a', 'control');
		INSERT INTO workflow_data_dependencies VALUES ('a', 'b', 'result.dat', 1024);
	`); err != nil {
		t.Fatal(err)
	}
	result := ports.ProvenanceLineage{}
	queue := []lineageVisit{}
	if err := New(db).expandActivityDependencies(
		context.Background(), lineageVisit{entity: "activities", id: "a"}, false,
		&queue, &result, map[string]bool{},
	); err != nil {
		t.Fatal(err)
	}
	if len(queue) != 2 || len(result.Edges) != 2 {
		t.Fatalf("unexpected activity lineage: %#v", result)
	}
}
