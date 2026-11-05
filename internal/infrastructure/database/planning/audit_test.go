package planning

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestPlanningSessionAndAlgorithmsPreserveLifecycleEvents(t *testing.T) {
	repo := setup(t)
	ctx := context.Background()
	session := domain.PlanningSession{ID: "audit-session", WorkflowVersionID: "w1", ExecutionScopeID: "scope", NetworkTopologyID: "network-v1"}
	runs := []domain.AlgorithmRun{{ID: "audit-algorithm", Algorithm: "test"}}
	if err := repo.CreateSession(ctx, session, runs); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := repo.SetSessionRunning(ctx, session.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.SetAlgorithmRunRunning(ctx, "audit-algorithm"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetAlgorithmRunFailed(ctx, "audit-algorithm", "secret-error"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetSessionFailed(ctx, session.ID, "secret-error"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := repo.db.QueryRow("SELECT COUNT(*) FROM audit_events WHERE source='planning'").Scan(&count); err != nil || count != 6 {
		t.Fatalf("%d %v", count, err)
	}
	if _, err := repo.db.Exec("DROP TABLE audit_events"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetSessionCompleted(ctx, session.ID); err == nil {
		t.Fatal("missing audit error")
	}
	stored, err := repo.FindSession(ctx, session.ID)
	if err != nil || string(stored.Status) != "failed" {
		t.Fatalf("%+v %v", stored, err)
	}
}
