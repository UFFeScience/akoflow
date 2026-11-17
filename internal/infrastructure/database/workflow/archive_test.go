package workflow

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestArchivePersistsWithoutRemovingWorkflowVersionsOrHistory(t *testing.T) {
	r := setupRepository(t)
	ctx := context.Background()
	for _, statement := range []string{
		`INSERT INTO workflow_definitions(id,external_id,name) VALUES ('w','w','Workflow')`,
		`INSERT INTO workflow_versions(id,workflow_id,version,definition_hash,status) VALUES ('v1','w',1,'old','published'),('v2','w',2,'new','published')`,
		`INSERT INTO execution_scopes(id,name) VALUES ('scope','Scope')`,
		`INSERT INTO schedule_plans(id,workflow_version_id,execution_scope_id,source,algorithm) VALUES ('plan','v2','scope','plugin','test')`,
		`INSERT INTO execution_runs(id,schedule_plan_id,mode,status) VALUES ('run','plan','real','completed')`,
	} {
		if _, err := r.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	for _, archived := range []bool{true, true, false, false} {
		if err := r.SetArchived(ctx, "w", archived); err != nil {
			t.Fatal(err)
		}
		reopened := New(r.db)
		definition, err := reopened.Find(ctx, "w")
		if err != nil || definition == nil || definition.Archived != archived {
			t.Fatalf("definition: %+v %v", definition, err)
		}
		header, err := reopened.FindHeader(ctx, "w")
		if err != nil || header.Archived != archived {
			t.Fatalf("header: %+v %v", header, err)
		}
		headers, err := reopened.ListHeaders(ctx)
		if err != nil || len(headers) != 1 || headers[0].Archived != archived {
			t.Fatalf("headers: %+v %v", headers, err)
		}
		for _, id := range []string{"v1", "v2"} {
			version, err := reopened.FindVersion(ctx, id)
			if err != nil || version.Archived != archived {
				t.Fatalf("version %s: %+v %v", id, version, err)
			}
		}
		catalog, err := reopened.ListCatalog(ctx)
		if err != nil || len(catalog) != 1 || catalog[0].Archived != archived || catalog[0].RunCount != 1 || catalog[0].PlanCount != 1 {
			t.Fatalf("catalog: %+v %v", catalog, err)
		}
	}
	if err := r.SetArchived(ctx, "missing", true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing workflow: %v", err)
	}
}
