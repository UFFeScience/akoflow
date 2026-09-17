package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type InterruptStore interface {
	FindRun(context.Context, string) (*domain.ExecutionRun, error)
	ListTasks(context.Context, string) ([]domain.TaskExecution, error)
	ListHandles(context.Context, string) ([]domain.ActivityHandle, error)
	SaveTask(context.Context, domain.TaskExecution) error
}

type ActivityStopper interface {
	Stop(context.Context, string, domain.ExecutionMode) error
}

// Interrupter stops the actual runtime before releasing an activity record.
// A task with no handle may only be reconciled after its run has terminated.
type Interrupter struct {
	Store   InterruptStore
	Stopper ActivityStopper
}

func (s Interrupter) Interrupt(ctx context.Context, runID, activityID string) (*domain.TaskExecution, error) {
	if s.Store == nil || s.Stopper == nil {
		return nil, fmt.Errorf("activity interruption is unavailable")
	}
	run, err := s.Store.FindRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("execution run %q not found", runID)
	}
	tasks, err := s.Store.ListTasks(ctx, runID)
	if err != nil {
		return nil, err
	}
	var task *domain.TaskExecution
	for index := range tasks {
		candidate := &tasks[index]
		if candidate.ActivityID == activityID && (task == nil || candidate.Attempt > task.Attempt) {
			task = candidate
		}
	}
	if task == nil {
		return nil, fmt.Errorf("activity %q has no execution record in run %q", activityID, runID)
	}
	if task.Status == domain.TaskCancelled {
		return task, nil
	}
	if task.Status == domain.TaskCompleted || task.Status == domain.TaskFailed {
		return nil, fmt.Errorf("activity %q is already %s", activityID, task.Status)
	}
	handles, err := s.Store.ListHandles(ctx, runID)
	if err != nil {
		return nil, err
	}
	var handle *domain.ActivityHandle
	for index := range handles {
		if handles[index].ActivityID == activityID {
			handle = &handles[index]
			break
		}
	}
	if handle == nil {
		if run.Status == domain.ExecutionRunRunning {
			return nil, fmt.Errorf("activity %q has no runtime handle while the run is active; wait for the scheduler before interrupting", activityID)
		}
	} else {
		switch handle.Status {
		case domain.HandleStarting, domain.HandleRunning:
			if err := s.Stopper.Stop(ctx, handle.ID, run.Mode); err != nil {
				return nil, fmt.Errorf("stop activity %q on its runtime: %w", activityID, err)
			}
		case domain.HandleStopped:
			// A prior request stopped the runtime; reconcile the task record.
		default:
			return nil, fmt.Errorf("activity %q has a %s runtime handle; inspect it before interrupting", activityID, handle.Status)
		}
	}
	task.Status = domain.TaskCancelled
	task.FinishedAt = float64(time.Now().UnixNano()) / float64(time.Second)
	task.FailureReason = "Interrupted by user before changing the machine lifecycle"
	if err := s.Store.SaveTask(ctx, *task); err != nil {
		return nil, fmt.Errorf("persist interrupted activity %q: %w", activityID, err)
	}
	return task, nil
}
