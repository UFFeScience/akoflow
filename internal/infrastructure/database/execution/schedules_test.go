package execution

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/application/recurring"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
)

func TestScheduleRollsBackAndEnqueuesOnceAcrossRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "schedules.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	t.Cleanup(func() { _ = repo.db.Close() })
	seedExecutionParents(t, repo)
	build := func(_ context.Context, schedule domainexecution.Schedule) (ports.ExecutionRequest, error) {
		return ports.ExecutionRequest{
			Run:      domain.ExecutionRun{SchedulePlanID: schedule.PlanID, Mode: domain.ExecutionModeReal, Status: domain.ExecutionRunCreated},
			Plan:     domain.SchedulePlan{ID: schedule.PlanID, WorkflowVersionID: schedule.WorkflowVersionID},
			Workflow: domain.WorkflowVersion{ID: schedule.WorkflowVersionID},
		}, nil
	}
	coordinator := recurring.Coordinator{Store: repo, Request: build}
	scheduledAt := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	schedule, err := coordinator.Create(ctx, domainexecution.Schedule{Name: "Daily", WorkflowVersionID: "workflow-version",
		PlanID: "plan", Frequency: "daily", LocalTime: "09:00", TimeZone: "UTC", MissedPolicy: "latest",
		OverlapPolicy: "skip"}, scheduledAt.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	verifyInterruptedSchedule(t, ctx, repo, coordinator, scheduledAt)
	var queueCount, runCount int
	if err := repo.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	repo = New(db)
	coordinator.Store = repo
	if _, err := repo.db.ExecContext(ctx, "INSERT INTO workflow_versions(id,workflow_id,version,definition_hash) VALUES('workflow-version-two','workflow',2,'new')"); err != nil {
		t.Fatal(err)
	}

	dispatchConcurrently(t, ctx, coordinator, scheduledAt.Add(2*time.Hour))
	var payload []byte
	if err := repo.db.QueryRowContext(ctx, "SELECT payload FROM queue_jobs").Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var request ports.ExecutionRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatal(err)
	}
	if request.Workflow.ID != schedule.WorkflowVersionID || request.Plan.ID != schedule.PlanID {
		t.Fatalf("dispatch lost pinned versions: %+v", request)
	}

	nextDay := scheduledAt.AddDate(0, 0, 1)
	if err := coordinator.Dispatch(ctx, nextDay); err != nil {
		t.Fatal(err)
	}
	current, err := repo.FindSchedule(ctx, schedule.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.LastOccurrence == nil || current.LastOccurrence.Outcome != "overlap" ||
		current.LastLaunch == nil || current.LastLaunch.RunID != request.Run.ID ||
		current.LastLaunch.RunStatus != domain.ExecutionRunCreated {
		t.Fatalf("unexpected schedule history: %+v", current)
	}
	if err := repo.db.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM queue_jobs),(SELECT COUNT(*) FROM execution_runs)").Scan(&queueCount, &runCount); err != nil ||
		queueCount != 1 || runCount != 1 {
		t.Fatalf("duplicate jobs=%d runs=%d error=%v", queueCount, runCount, err)
	}
	verifyPauseAndResume(t, ctx, coordinator, schedule.ID, nextDay)
}

func dispatchConcurrently(t *testing.T, ctx context.Context, coordinator recurring.Coordinator, now time.Time) {
	t.Helper()
	var group sync.WaitGroup
	for range 2 {
		group.Go(func() {
			if err := coordinator.Dispatch(ctx, now); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
}

func verifyInterruptedSchedule(t *testing.T, ctx context.Context, repo *Repository, coordinator recurring.Coordinator, scheduledAt time.Time) {
	t.Helper()
	if _, err := repo.db.ExecContext(ctx, `CREATE TRIGGER interrupt_occurrence BEFORE INSERT ON execution_schedule_occurrences
		BEGIN SELECT RAISE(ABORT,'interrupted before occurrence commit'); END`); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Dispatch(ctx, scheduledAt); err == nil {
		t.Fatal("expected rollback")
	}
	var queueCount, runCount int
	if err := repo.db.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM queue_jobs),(SELECT COUNT(*) FROM execution_runs)").Scan(&queueCount, &runCount); err != nil ||
		queueCount != 0 || runCount != 0 {
		t.Fatalf("partial commit jobs=%d runs=%d error=%v", queueCount, runCount, err)
	}
	if _, err := repo.db.ExecContext(ctx, "DROP TRIGGER interrupt_occurrence"); err != nil {
		t.Fatal(err)
	}
}

func verifyPauseAndResume(t *testing.T, ctx context.Context, coordinator recurring.Coordinator, scheduleID string, nextDay time.Time) {
	t.Helper()
	if _, err := coordinator.SetPaused(ctx, scheduleID, true, nextDay); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Dispatch(ctx, nextDay.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	resumedAt := nextDay.AddDate(0, 0, 2).Add(time.Hour)
	resumed, err := coordinator.SetPaused(ctx, scheduleID, false, resumedAt)
	if err != nil || resumed.Paused || !resumed.NextAt.Equal(nextDay.AddDate(0, 0, 3)) {
		t.Fatalf("resume did not begin at the next future occurrence: %+v %v", resumed, err)
	}
}

func TestScheduleAllowsOverlapAndSkipsMissedTimes(t *testing.T) {
	ctx := context.Background()
	repo := setup(t)
	coordinator := recurring.Coordinator{Store: repo, Request: func(_ context.Context, s domainexecution.Schedule) (ports.ExecutionRequest, error) {
		return ports.ExecutionRequest{Run: domain.ExecutionRun{SchedulePlanID: s.PlanID, Mode: domain.ExecutionModeReal, Status: domain.ExecutionRunCreated}}, nil
	}}
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	schedule, err := coordinator.Create(ctx, domainexecution.Schedule{Name: "Daily", WorkflowVersionID: "workflow-version",
		PlanID: "plan", Frequency: "daily", LocalTime: "09:00", TimeZone: "UTC", MissedPolicy: "skip",
		OverlapPolicy: "allow"}, now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Dispatch(ctx, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	missed, err := repo.FindSchedule(ctx, schedule.ID)
	if err != nil || missed.LastOccurrence == nil || missed.LastOccurrence.Outcome != "missed" || missed.LastLaunch != nil {
		t.Fatalf("unexpected missed launch: %+v %v", missed, err)
	}
	for day := 1; day <= 2; day++ {
		if err := coordinator.Dispatch(ctx, now.AddDate(0, 0, day)); err != nil {
			t.Fatal(err)
		}
	}
	var runs int
	if err := repo.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM execution_runs WHERE status='created'").Scan(&runs); err != nil || runs != 2 {
		t.Fatalf("allow overlap created %d pending runs, error=%v", runs, err)
	}
}
