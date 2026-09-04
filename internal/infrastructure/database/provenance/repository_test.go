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
