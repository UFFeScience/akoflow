package execution

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestRunFeedJoinsOnlyTheSelectedPage(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := r.CreateRun(ctx, domain.ExecutionRun{ID: fmt.Sprintf("run-%d", i), SchedulePlanID: "plan", Mode: domain.ExecutionModeReal, Status: domain.ExecutionRunCompleted}); err != nil {
			t.Fatal(err)
		}
		if _, err := r.db.Exec(`INSERT INTO task_executions(id,execution_run_id,plan_assignment_id,activity_id,planned_resource_id,status,runtime_seconds,queue_seconds)
			VALUES (?,?, 'assignment','activity','resource','completed',?,2)`, fmt.Sprintf("task-%d", i), fmt.Sprintf("run-%d", i), i+1); err != nil {
			t.Fatal(err)
		}
		if _, err := r.db.Exec(`INSERT INTO data_transfer_observations(id,execution_run_id,source_resource_id,target_resource_id,status,bytes,duration_seconds)
			VALUES (?,?,'resource','resource','completed',?,3)`, fmt.Sprintf("transfer-%d", i), fmt.Sprintf("run-%d", i), (i+1)*100); err != nil {
			t.Fatal(err)
		}
	}
	page, err := r.ListRunsPage(ctx, 2, 1, "workflow", "real", "completed", "workflow-version", "plan")
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || !page.HasNext || len(page.Items) != 1 {
		t.Fatalf("page=%+v", page)
	}
	run := page.Items[0]
	if run.ID != "run-1" || run.ActivityCount != 1 || run.CompletedActivityCount != 1 || run.Breakdown.ComputeSeconds != 2 || run.Breakdown.TransferSeconds != 3 || run.TransferredBytes != 200 {
		t.Fatalf("incorrect page aggregates: %+v", run)
	}
	if run.WorkflowID != "workflow" || run.WorkflowVersionID != "workflow-version" || run.WorkflowName != "test" || len(run.EnvironmentNames) != 1 || run.EnvironmentNames[0] != "test" {
		t.Fatalf("missing joined context: %+v", run)
	}
	empty, err := r.ListRunsPage(ctx, 1, 20, "", "", "", "", "missing-plan")
	if err != nil || empty.Total != 0 || len(empty.Items) != 0 {
		t.Fatalf("plan filter: %+v, %v", empty, err)
	}
	// Guard the query plan: related history must use keyed searches, rather
	// than scanning task/transfer tables for every run in the database.
	rows, err := r.db.Query(`EXPLAIN QUERY PLAN ` + runFeedPageQuery(`SELECT * FROM (`+runFeedSelect+`) ORDER BY started_at DESC,id DESC LIMIT 20 OFFSET 20`))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var details string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		details += detail + "\n"
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"task_executions", "data_transfer_observations"} {
		if !strings.Contains(details, "SEARCH "+table+" USING INDEX") || strings.Contains(details, "SCAN "+table) {
			t.Fatalf("unbounded aggregate for %s:\n%s", table, details)
		}
	}
}
