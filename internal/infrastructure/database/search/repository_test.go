package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	domainsearch "github.com/UFFeScience/akoflow/internal/domain/search"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
)

func searchDatabase(t testing.TB) *sql.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO environments(id,name) VALUES ('lab','Science lab');
 INSERT INTO workflow_definitions(id,external_id,name,namespace) VALUES ('wf','external-wf','Science workflow','research');
 INSERT INTO workflow_versions(id,workflow_id,version,definition_hash) VALUES ('v1','wf',1,'hash');
 INSERT INTO execution_scopes(id,name) VALUES ('scope','Science scope');
 INSERT INTO schedule_plans(id,workflow_version_id,execution_scope_id,source,algorithm)
 VALUES ('plan','v1','scope','manual','Science scheduler');
 INSERT INTO artifact_versions VALUES ('av1','artifact','Science tool','1','workflow','v1');
 INSERT INTO execution_runs(id,schedule_plan_id,mode,status) VALUES ('run','plan','real','completed');
 UPDATE workflow_versions SET raw_definition='secret-value';
 UPDATE environments SET description='secret-value';
 UPDATE execution_runs SET failure_reason='secret-value';`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSearchMatchesMetadataAndRoutesToOwners(t *testing.T) {
	db := searchDatabase(t)
	page, err := New(db).Search(context.Background(), domainsearch.Query{Text: "sCiEnCe", Types: domainsearch.Types, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	byType := map[string]domainsearch.Result{}
	for _, item := range page.Results {
		byType[item.Type] = item
	}
	for _, kind := range []string{"workflow", "environment", "execution", "artifact", "plan", "scope"} {
		if byType[kind].ID == "" {
			t.Errorf("missing %s: %+v", kind, page)
		}
	}
	if got := byType["execution"]; got.OwnerTitle != "Science workflow" || got.Path != "/workflows/wf/plans/plan/executions/run" {
		t.Fatalf("execution hierarchy: %+v", got)
	}
	if got := byType["artifact"]; got.Path != "/artifacts/artifact" || got.Version != "1" || got.OwnerPath != "/workflows/wf" {
		t.Fatalf("artifact: %+v", got)
	}
	if byType["workflow"].Score <= byType["execution"].Score {
		t.Fatal("direct name matches must rank ahead of owner-only matches")
	}
	exact, err := New(db).Search(context.Background(), domainsearch.Query{Text: "external-wf", Types: []string{"workflow"}, Limit: 1})
	if err != nil || len(exact.Results) != 1 || exact.Results[0].Score != 3 {
		t.Fatalf("exact ID: %+v %v", exact, err)
	}
	for _, needle := range []string{"secret-value", "%", "' OR 1=1 --"} {
		page, err := New(db).Search(context.Background(), domainsearch.Query{Text: needle, Types: domainsearch.Types, Limit: 20})
		if err != nil || len(page.Results) != 0 {
			t.Fatalf("unexpected match %q: %+v %v", needle, page, err)
		}
	}
}

func TestArtifactOwnerResolvesVersionAndLegacyWorkflowIdentifiers(t *testing.T) {
	db := searchDatabase(t)
	for _, scope := range []string{"v1", "wf", "external-wf"} {
		if _, err := db.Exec(`UPDATE artifact_versions SET scope_id=? WHERE id='av1'`, scope); err != nil {
			t.Fatal(err)
		}
		page, err := New(db).Search(context.Background(), domainsearch.Query{Text: "artifact", Types: []string{"artifact"}, Limit: 20})
		if err != nil || len(page.Results) != 1 {
			t.Fatalf("scope %s: %+v %v", scope, page, err)
		}
		if page.Results[0].OwnerID != "wf" || page.Results[0].OwnerPath != "/workflows/wf" {
			t.Fatalf("unresolved owner for %s: %+v", scope, page.Results[0])
		}
	}
}

func TestSearchCursorIsBoundToQueryAndDoesNotSkipEqualNames(t *testing.T) {
	db := searchDatabase(t)
	for i := 0; i < 7; i++ {
		if _, err := db.Exec(`INSERT INTO environments(id,name) VALUES (?, 'Same name')`, fmt.Sprintf("env-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	repo := New(db)
	query := domainsearch.Query{Text: "Same", Types: []string{"environment"}, Limit: 2}
	seen := map[string]bool{}
	for {
		page, err := repo.Search(context.Background(), query)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Results) > 2 {
			t.Fatal("unbounded page")
		}
		for _, item := range page.Results {
			if seen[item.ID] {
				t.Fatal("duplicate", item.ID)
			}
			seen[item.ID] = true
		}
		if page.NextCursor == "" {
			break
		}
		bad := query
		bad.Cursor = page.NextCursor
		bad.Text = "different"
		if _, err := repo.Search(context.Background(), bad); !errors.Is(err, domainsearch.ErrInvalidCursor) {
			t.Fatalf("cursor reused: %v", err)
		}
		query.Cursor = page.NextCursor
	}
	if len(seen) != 7 {
		t.Fatalf("found %d records", len(seen))
	}
	query.Cursor = "invalid"
	if _, err := repo.Search(context.Background(), query); !errors.Is(err, domainsearch.ErrInvalidCursor) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	query.Cursor = ""
	if _, err := repo.Search(ctx, query); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func BenchmarkSearchLargeRunHistory(b *testing.B) {
	db := searchDatabase(b)
	_, err := db.Exec(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i<100000)
 INSERT INTO execution_runs(id,schedule_plan_id,mode,status,failure_reason)
 SELECT printf('run-%06d',i),'plan','real','completed',zeroblob(1024) FROM n`)
	if err != nil {
		b.Fatal(err)
	}
	repo := New(db)
	for _, text := range []string{"run-050000", "Science", "missing-name"} {
		b.Run(text, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				page, err := repo.Search(context.Background(), domainsearch.Query{Text: text, Types: domainsearch.Types, Limit: 20})
				if err != nil {
					b.Fatal(err)
				}
				if len(page.Results) > 20 {
					b.Fatal("unbounded results")
				}
			}
		})
	}
}
