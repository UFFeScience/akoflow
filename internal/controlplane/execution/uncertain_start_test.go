package execution

import (
	"context"
	"fmt"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type uncertainStartFailure struct{}

func (uncertainStartFailure) Error() string { return "submission outcome uncertain" }
func (uncertainStartFailure) RuntimeReconciliationMetadata() map[string]any {
	return map[string]any{"slurmSubmissionName": "attempt-bound-name"}
}

func TestUncertainStartFailureKeepsHandleActiveForRecoveryGate(t *testing.T) {
	store := &executionStoreFake{}
	s := &Supervisor{executions: store}
	err := s.recordStartFailure(context.Background(), "run", "activity", domain.PlanAssignment{},
		domain.Resource{ID: "hpc"}, "slurm", 2, fmt.Errorf("wrapped: %w", uncertainStartFailure{}))
	if err != nil {
		t.Fatal(err)
	}
	handle := store.handles["run:activity:attempt-2"]
	if handle.Status != domain.HandleStarting || handle.FinishedAt != 0 {
		t.Fatalf("unconfirmed runtime must remain active, not recoverable: %+v", handle)
	}
	if len(store.tasks) != 1 || store.tasks[0].Status != domain.TaskFailed {
		t.Fatalf("failed attempt must still be recorded: %+v", store.tasks)
	}
}
