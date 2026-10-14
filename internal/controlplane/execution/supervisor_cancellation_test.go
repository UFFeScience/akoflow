package execution

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type cancelledExecutionPlan struct{}

type cleanupContextController struct {
	activityControllerFake
	err error
}

func (c *cleanupContextController) Stop(ctx context.Context, _ string, _ domain.ExecutionMode) error {
	c.err = ctx.Err()
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("cleanup has no bounded deadline")
	}
	return c.err
}

func TestFailureCleanupUsesBoundedUncancelledContext(t *testing.T) {
	store := &executionStoreFake{}
	controller := &cleanupContextController{}
	s, _ := New(store, controller, &planExecutorFake{}, Config{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	running := map[string]domain.ActivityHandle{"a": {ID: "h"}}
	tasks := map[string]domain.TaskExecution{"a": {ActivityID: "a", Status: domain.TaskRunning}}
	s.cancelRunningActivities(ctx, domain.ExecutionModeReal, running, tasks, context.Canceled)
	if controller.err != nil || len(running) != 0 || tasks["a"].Status != domain.TaskCancelled {
		t.Fatalf("cleanup err=%v running=%v task=%+v", controller.err, running, tasks["a"])
	}
}

func (cancelledExecutionPlan) Execute(ctx context.Context, _ ports.ExecutionRequest) (domain.ExecutionTrace, error) {
	return domain.ExecutionTrace{}, ctx.Err()
}

func TestSupervisorPersistsLeaseCancellationCause(t *testing.T) {
	store := &executionStoreFake{}
	service, err := New(store, &activityControllerFake{}, cancelledExecutionPlan{}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errors.New("renew lease for queue job example: lease ownership lost"))
	_, err = service.Execute(ctx, requestFixture(domain.ExecutionModeSimulation))
	if !errors.Is(err, context.Canceled) || !strings.Contains(store.failed, "lease ownership lost") {
		t.Fatalf("lease cancellation hidden from run: err=%v failure=%q", err, store.failed)
	}
}
