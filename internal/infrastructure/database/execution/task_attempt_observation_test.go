package execution

import (
	"context"
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestOldStoppedHandleDoesNotOverrideRecoveredRunningTask(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	if err := r.CreateRun(ctx, domain.ExecutionRun{ID: "run", SchedulePlanID: "plan", Mode: domain.ExecutionModeReal, Status: domain.ExecutionRunRunning}); err != nil {
		t.Fatal(err)
	}
	for attempt, id := range []string{"run:activity", "run:activity:attempt-2"} {
		task := domain.TaskExecution{ID: id, ExecutionRunID: "run", PlanAssignmentID: "assignment", ActivityID: "activity", PlannedResourceID: "resource", Attempt: attempt + 1, Status: domain.TaskRunning, StartedAt: float64(10 + 10*attempt)}
		if err := r.SaveTask(ctx, task); err != nil {
			t.Fatal(err)
		}
		handle := domain.ActivityHandle{ID: id, RunID: "run", ActivityID: "activity", ResourceID: "resource", RuntimeID: "local", Status: domain.HandleRunning}
		if attempt == 0 {
			handle.Status = domain.HandleStopped
			handle.Failure = "old attempt cancelled"
			handle.FinishedAt = 12
		}
		if err := r.Save(ctx, handle); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := r.ListTasks(ctx, "run")
	if err != nil || len(tasks) != 2 {
		t.Fatalf("tasks=%+v err=%v", tasks, err)
	}
	if tasks[0].Status != domain.TaskFailed || tasks[0].FailureReason != "old attempt cancelled" {
		t.Fatalf("old attempt: %+v", tasks[0])
	}
	if tasks[1].Status != domain.TaskRunning || tasks[1].FailureReason != "" || tasks[1].FinishedAt != 0 {
		t.Fatalf("recovered attempt contaminated: %+v", tasks[1])
	}
}
