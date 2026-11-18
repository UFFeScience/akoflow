package execution

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestSharedLocalCapacityAcrossRuns(t *testing.T) {
	limit := ports.LocalActivityCapacity{CPU: 8, MemoryBytes: 8 << 30}
	for dimension, request := range map[string]ports.LocalActivityCapacity{
		"CPU":    {CPU: 6, MemoryBytes: 1 << 30},
		"memory": {CPU: 1, MemoryBytes: 6 << 30},
	} {
		t.Run(dimension, func(t *testing.T) {
			repository := setup(t)
			first := localAdmissionTask(t, repository, "first")
			second := localAdmissionTask(t, repository, "second")
			assertReservation(t, repository, &first, request, limit, true)
			assertReservation(t, repository, &second, request, limit, false)

			first.Status, first.Metadata = domain.TaskRunning, nil
			if err := repository.SaveTask(context.Background(), first); err != nil {
				t.Fatal(err)
			}
			assertReservation(t, repository, &second, request, limit, false)

			first.Status = domain.TaskCompleted
			if err := repository.SaveTask(context.Background(), first); err != nil {
				t.Fatal(err)
			}
			assertReservation(t, repository, &second, request, limit, true)
		})
	}
}

func TestRecoverLocalPreparationsKeepsUncertainLaunchReserved(t *testing.T) {
	ctx := context.Background()
	repository := setup(t)
	first := localAdmissionTask(t, repository, "first")
	second := localAdmissionTask(t, repository, "second")
	capacity := ports.LocalActivityCapacity{CPU: 8, MemoryBytes: 8 << 30}
	assertReservation(t, repository, &first, capacity, capacity, true)
	if err := repository.RecoverLocalPreparations(ctx); err != nil {
		t.Fatal(err)
	}
	assertReservation(t, repository, &second, capacity, capacity, true)
	if err := repository.MarkLocalActivityLaunching(ctx, second.ExecutionRunID, second.ActivityID, second.Attempt, true); err != nil {
		t.Fatal(err)
	}
	if err := repository.RecoverLocalPreparations(ctx); err != nil {
		t.Fatal(err)
	}
	assertReservation(t, repository, &first, capacity, capacity, false)
	tasks, err := repository.ListLocalReconciliations(ctx)
	if err != nil || len(tasks) != 1 || tasks[0].TaskID != second.ID || !tasks[0].Docker {
		t.Fatalf("incorrect reconciliation candidates: %v, %v", tasks, err)
	}
	if err := repository.ReleaseReconciledLocalActivity(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	assertReservation(t, repository, &first, capacity, capacity, true)
}

func localAdmissionTask(t *testing.T, repository *Repository, runID string) domain.TaskExecution {
	t.Helper()
	run := domain.ExecutionRun{ID: runID, SchedulePlanID: "plan", Mode: domain.ExecutionModeReal, Status: domain.ExecutionRunRunning}
	if err := repository.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	return domain.TaskExecution{
		ID: runID + ":task", ExecutionRunID: runID, PlanAssignmentID: "assignment",
		ActivityID: "activity", PlannedResourceID: "resource", Attempt: 1, Status: domain.TaskQueued,
	}
}

func assertReservation(t *testing.T, repository *Repository, task *domain.TaskExecution, request, limit ports.LocalActivityCapacity, want bool) {
	t.Helper()
	got, err := repository.ReserveLocalActivity(context.Background(), task, request, limit)
	if err != nil || got != want {
		t.Fatalf("reservation for %s: got %v, want %v, error %v", task.ExecutionRunID, got, want, err)
	}
}
