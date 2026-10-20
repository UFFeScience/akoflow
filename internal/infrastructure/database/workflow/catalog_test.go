package workflow

import (
	"context"
	"fmt"
	"testing"
)

func TestCatalogCountsAllRunsButReturnsThreeHeadersForLatestVersion(t *testing.T) {
	r := setupRepository(t)
	ctx := context.Background()
	for _, statement := range []string{
		`INSERT INTO workflow_definitions(id,external_id,name) VALUES ('w','w','Workflow'),('empty','empty','Empty')`,
		`INSERT INTO workflow_versions(id,workflow_id,version,definition_hash) VALUES ('v1','w',1,'1'),('v2','w',2,'2'),('v-empty','empty',1,'e')`,
		`INSERT INTO execution_scopes(id,name) VALUES ('scope','Scope')`,
		`INSERT INTO schedule_plans(id,workflow_version_id,execution_scope_id,source,algorithm) VALUES ('old','v1','scope','plugin','test'),('p','v2','scope','plugin','test'),('unused','v2','scope','plugin','test')`,
		`INSERT INTO execution_runs(id,schedule_plan_id,mode,status,cost,started_at) VALUES ('old-run','old','real','completed',99,'2099-01-01 00:00:00')`,
	} {
		if _, err := r.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 5; i++ {
		if _, err := r.db.Exec(`INSERT INTO execution_runs(id,schedule_plan_id,mode,status,cost,started_at) VALUES (?,'p','real',?,2,?)`,
			fmt.Sprintf("r%d", i), []string{"failed", "completed", "completed", "completed", "running"}[i], fmt.Sprintf("2026-01-01 00:00:0%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := r.ListCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].RunCount != 0 || len(entries[0].RecentRuns) != 0 {
		t.Fatalf("empty workflow: %+v", entries)
	}
	e := entries[1]
	if e.Version.ID != "v2" || e.PlanCount != 2 || e.RunCount != 5 || e.CompletedRunCount != 3 || e.ObservedCost != 10 || len(e.RecentRuns) != 3 || e.RecentRuns[0].Run.ID != "r4" || e.RecentRuns[2].Run.ID != "r2" || e.LatestExecutionAt != e.RecentRuns[0].Run.StartedAt.UnixMilli() {
		t.Fatalf("incorrect catalog summary: %+v", e)
	}
}
