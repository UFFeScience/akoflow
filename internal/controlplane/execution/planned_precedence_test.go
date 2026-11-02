package execution

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type plannedCompletionController struct {
	activityControllerFake
	mu        sync.Mutex
	required  map[string]string
	completed map[string]bool
}

func (c *plannedCompletionController) Start(ctx context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	c.mu.Lock()
	previous := c.required[execution.Activity.ID]
	allowed := previous == "" || c.completed[previous]
	c.mu.Unlock()
	if !allowed {
		return domain.ActivityHandle{}, fmt.Errorf("%s started before planned predecessor %s completed", execution.Activity.ID, previous)
	}
	return c.activityControllerFake.Start(ctx, execution)
}

func (c *plannedCompletionController) Inspect(ctx context.Context, id string, mode domain.ExecutionMode) (*domain.ActivityHandle, error) {
	handle, err := c.activityControllerFake.Inspect(ctx, id, mode)
	if err == nil && handle != nil && handle.Status == domain.HandleCompleted {
		c.mu.Lock()
		if c.completed == nil {
			c.completed = make(map[string]bool)
		}
		c.completed[strings.TrimPrefix(id, "h-")] = true
		c.mu.Unlock()
	}
	return handle, err
}

func TestSupervisorRejectsLaneDependencyCycleBeforeRunCreation(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	request.Plan.Assignments[0].OrderOnResource = 1
	controller := &activityControllerFake{}
	store := &executionStoreFake{}
	service, err := New(store, controller, &planExecutorFake{}, Config{PollInterval: time.Microsecond})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Execute(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "planned lane order") {
		t.Fatalf("expected combined lane/workflow cycle rejection, got %v", err)
	}
	if len(store.runs) != 0 || len(controller.started) != 0 {
		t.Fatal("inconsistent plan created a run or started activities")
	}
}
