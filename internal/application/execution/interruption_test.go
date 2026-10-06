package execution

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type interruptionStoreFake struct {
	run     *domain.ExecutionRun
	task    domain.TaskExecution
	handles []domain.ActivityHandle
	saved   *domain.TaskExecution
}

func (f *interruptionStoreFake) FindRun(context.Context, string) (*domain.ExecutionRun, error) {
	return f.run, nil
}
func (f *interruptionStoreFake) ListTasks(context.Context, string) ([]domain.TaskExecution, error) {
	return []domain.TaskExecution{f.task}, nil
}
func (f *interruptionStoreFake) ListHandles(context.Context, string) ([]domain.ActivityHandle, error) {
	return f.handles, nil
}
func (f *interruptionStoreFake) SaveTask(_ context.Context, task domain.TaskExecution) error {
	f.saved = &task
	return nil
}

type interruptionStopperFake struct {
	called bool
	err    error
	ids    []string
}

func (f *interruptionStopperFake) Stop(_ context.Context, id string, _ domain.ExecutionMode) error {
	f.called = true
	f.ids = append(f.ids, id)
	return f.err
}

func TestInterruptStopsEveryActiveAttemptIgnoringCompletedHistory(t *testing.T) {
	store := &interruptionStoreFake{
		run:  &domain.ExecutionRun{ID: "run", Status: domain.ExecutionRunFailed, Mode: domain.ExecutionModeReal},
		task: domain.TaskExecution{ActivityID: "activity", Status: domain.TaskCancelled},
		handles: []domain.ActivityHandle{
			{ID: "old", ActivityID: "activity", Status: domain.HandleCompleted},
			{ID: "attempt-2", ActivityID: "activity", Status: domain.HandleRunning},
			{ID: "attempt-3", ActivityID: "activity", Status: domain.HandleStarting},
			{ID: "other", ActivityID: "other", Status: domain.HandleRunning},
		},
	}
	stopper := &interruptionStopperFake{}
	_, err := (Interrupter{Store: store, Stopper: stopper}).Interrupt(context.Background(), "run", "activity")
	if err != nil || len(stopper.ids) != 2 || stopper.ids[0] != "attempt-2" || stopper.ids[1] != "attempt-3" {
		t.Fatalf("stopped=%v err=%v", stopper.ids, err)
	}
}

func TestInterruptStopsRuntimeBeforeReleasingTask(t *testing.T) {
	store := &interruptionStoreFake{
		run:     &domain.ExecutionRun{ID: "run", Status: domain.ExecutionRunFailed, Mode: domain.ExecutionModeReal},
		task:    domain.TaskExecution{ID: "task", ExecutionRunID: "run", ActivityID: "activity", Attempt: 1, Status: domain.TaskRunning},
		handles: []domain.ActivityHandle{{ID: "run:activity", RunID: "run", ActivityID: "activity", Status: domain.HandleRunning}},
	}
	stopper := &interruptionStopperFake{}
	task, err := (Interrupter{Store: store, Stopper: stopper}).Interrupt(context.Background(), "run", "activity")
	if err != nil || !stopper.called || task.Status != domain.TaskCancelled || store.saved == nil || store.saved.FinishedAt <= 0 {
		t.Fatalf("task=%+v saved=%+v stopped=%v err=%v", task, store.saved, stopper.called, err)
	}
}

func TestInterruptKeepsTaskActiveWhenRuntimeStopFails(t *testing.T) {
	store := &interruptionStoreFake{
		run:     &domain.ExecutionRun{ID: "run", Status: domain.ExecutionRunFailed, Mode: domain.ExecutionModeReal},
		task:    domain.TaskExecution{ID: "task", ExecutionRunID: "run", ActivityID: "activity", Attempt: 1, Status: domain.TaskRunning},
		handles: []domain.ActivityHandle{{ID: "run:activity", RunID: "run", ActivityID: "activity", Status: domain.HandleRunning}},
	}
	stopper := &interruptionStopperFake{err: errors.New("SSH unavailable")}
	_, err := (Interrupter{Store: store, Stopper: stopper}).Interrupt(context.Background(), "run", "activity")
	if err == nil || !strings.Contains(err.Error(), "SSH unavailable") || store.saved != nil {
		t.Fatalf("unexpected interruption result: err=%v saved=%+v", err, store.saved)
	}
}

func TestInterruptStopsHandleEvenWhenTaskWasAlreadyCancelled(t *testing.T) {
	store := &interruptionStoreFake{
		run:     &domain.ExecutionRun{ID: "run", Status: domain.ExecutionRunFailed, Mode: domain.ExecutionModeReal},
		task:    domain.TaskExecution{ID: "task", ActivityID: "activity", Status: domain.TaskCancelled},
		handles: []domain.ActivityHandle{{ID: "run:activity", ActivityID: "activity", Status: domain.HandleStarting}},
	}
	stopper := &interruptionStopperFake{}
	_, err := (Interrupter{Store: store, Stopper: stopper}).Interrupt(context.Background(), "run", "activity")
	if err != nil || !stopper.called {
		t.Fatalf("stop called=%v err=%v", stopper.called, err)
	}
}

func TestInterruptRequiresHandleWhileRunIsActive(t *testing.T) {
	store := &interruptionStoreFake{
		run:  &domain.ExecutionRun{ID: "run", Status: domain.ExecutionRunRunning},
		task: domain.TaskExecution{ID: "task", ExecutionRunID: "run", ActivityID: "activity", Attempt: 1, Status: domain.TaskPreparing},
	}
	_, err := (Interrupter{Store: store, Stopper: &interruptionStopperFake{}}).Interrupt(context.Background(), "run", "activity")
	if err == nil || store.saved != nil {
		t.Fatalf("active run without handle was incorrectly released: %v", err)
	}
}
