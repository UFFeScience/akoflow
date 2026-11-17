package execution

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type failingStateStore struct {
	executionStoreFake
}

func (*failingStateStore) SaveTask(context.Context, domain.TaskExecution) error {
	return errors.New("task database unavailable")
}
func (*failingStateStore) FailRun(context.Context, string, string) error {
	return errors.New("run database unavailable")
}

func TestCancellationPersistenceFailureRetainsPendingTask(t *testing.T) {
	store := &failingStateStore{}
	service, _ := New(store, &activityControllerFake{}, &planExecutorFake{}, Config{})
	running := map[string]domain.ActivityHandle{"a": {ID: "h"}}
	tasks := map[string]domain.TaskExecution{"a": {ActivityID: "a", Status: domain.TaskRunning}}
	err := service.cancelRunningActivities(context.Background(), domain.ExecutionModeReal, running, tasks, errors.New("original failure"))
	if err == nil || !strings.Contains(err.Error(), "task database unavailable") || len(running) != 1 || tasks["a"].Status != domain.TaskRunning {
		t.Fatalf("persistence failure hidden: %v %+v", err, tasks)
	}
}

func TestRunFailureRetainsOriginalAndPersistenceErrors(t *testing.T) {
	service, _ := New(&failingStateStore{}, &activityControllerFake{}, cancelledExecutionPlan{}, Config{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.Execute(ctx, requestFixture(domain.ExecutionModeSimulation))
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "run database unavailable") {
		t.Fatalf("lost failure: %v", err)
	}
}

func TestTerminalTaskPersistenceFailureDoesNotCommitInMemory(t *testing.T) {
	service := &Supervisor{executions: &failingStateStore{}}
	running := map[string]domain.ActivityHandle{"a": {ID: "h"}}
	tasks := map[string]domain.TaskExecution{"a": {ActivityID: "a", Status: domain.TaskRunning}}
	result := inspectionResult{activityID: "a", observed: &domain.ActivityHandle{Status: domain.HandleFailed, Failure: "exit 42"}}
	err := service.applyInspectionResult(context.Background(), domain.WorkflowVersion{}, "run", result, running, map[string]domain.TaskExecution{}, tasks, nil)
	if err == nil || !strings.Contains(err.Error(), "exit 42") || !strings.Contains(err.Error(), "database unavailable") || len(running) != 1 || tasks["a"].Status != domain.TaskRunning {
		t.Fatalf("lost terminal persistence: %v", err)
	}
}
