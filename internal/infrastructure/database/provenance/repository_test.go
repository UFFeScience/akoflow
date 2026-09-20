package provenance

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database/schema"
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
	plan, err := repository.Explain(context.Background(), ports.ProvenanceSQLQuery{
		SQL: "SELECT id, status FROM execution_runs WHERE status = :status", Parameters: map[string]any{"status": "completed"},
	})
	if err != nil || len(plan.Items) == 0 {
		t.Fatalf("unexpected SQL explanation: %#v, %v", plan, err)
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

func TestSchemaOmitsProtectedColumnsAndAdvertisedColumnsCanBeQueried(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE activity_resource_profiles (
			id TEXT,
			activity_type_id TEXT,
			metadata TEXT
		);
		INSERT INTO activity_resource_profiles VALUES ('profile-1', 'type-1', '{"secret":true}');
	`); err != nil {
		t.Fatal(err)
	}

	repository := New(db)
	tables, err := repository.Schema(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		if table.Name != "activity_resource_profiles" {
			continue
		}
		if len(table.Columns) != 2 || table.Columns[0].Name != "id" || table.Columns[1].Name != "activity_type_id" {
			t.Fatalf("protected columns leaked into safe schema: %#v", table.Columns)
		}
		result, err := repository.SQL(context.Background(), ports.ProvenanceSQLQuery{
			SQL: `SELECT "id", "activity_type_id" FROM "activity_resource_profiles"`,
		})
		if err != nil {
			t.Fatalf("query advertised columns: %v", err)
		}
		if len(result.Items) != 1 || result.Items[0]["id"] != "profile-1" {
			t.Fatalf("unexpected SQL result: %#v", result)
		}
		if _, exists := result.Items[0]["metadata"]; exists {
			t.Fatalf("protected metadata returned: %#v", result.Items[0])
		}
		return
	}
	t.Fatal("activity_resource_profiles missing from safe schema")
}

func TestSafeSQLSchemaMatchesCanonicalSchema(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err := db.Exec(schema.SQL); err != nil {
		t.Fatal(err)
	}

	repository := New(db)
	tables, err := repository.Schema(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != len(safeSQLSchema) {
		t.Fatalf("safe schema returned %d tables, want %d", len(tables), len(safeSQLSchema))
	}
	for _, table := range tables {
		if len(table.Columns) == 0 {
			t.Fatalf("safe table %q has no columns", table.Name)
		}
		columns := make([]string, 0, len(table.Columns))
		for _, column := range table.Columns {
			columns = append(columns, quotedIdentifier(column.Name))
		}
		if _, err := repository.SQL(context.Background(), ports.ProvenanceSQLQuery{
			SQL: "SELECT " + strings.Join(columns, ", ") + " FROM " + quotedIdentifier(table.Name) + " LIMIT 0",
		}); err != nil {
			t.Fatalf("query advertised columns from %q: %v", table.Name, err)
		}
	}

	blockedQueries := []string{
		"SELECT metadata FROM activity_resource_profiles",
		"SELECT credential_reference FROM storage_resources",
		"SELECT request FROM cloud_operation_runs",
		"SELECT raw FROM cloud_operation_events",
		"SELECT terraform_output FROM cloud_provisioned_instances",
		"SELECT logs FROM build_runs",
	}
	for _, query := range blockedQueries {
		if _, err := repository.SQL(context.Background(), ports.ProvenanceSQLQuery{SQL: query}); err == nil {
			t.Fatalf("protected query was allowed: %s", query)
		}
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
