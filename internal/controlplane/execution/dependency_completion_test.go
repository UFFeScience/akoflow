package execution

import (
	"context"
	"fmt"
	"sync"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// Observe the dependency barrier, not unrelated goroutine startup ordering.
type dependencyCompletionController struct {
	activityControllerFake
	completionMu      sync.Mutex
	producerCompleted bool
}

func (f *dependencyCompletionController) Start(ctx context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	f.completionMu.Lock()
	completed := f.producerCompleted
	f.completionMu.Unlock()
	if execution.Activity.ID == "a" && !completed {
		return domain.ActivityHandle{}, fmt.Errorf("consumer started before producer completion")
	}
	return f.activityControllerFake.Start(ctx, execution)
}

func (f *dependencyCompletionController) Inspect(ctx context.Context, id string, mode domain.ExecutionMode) (*domain.ActivityHandle, error) {
	handle, err := f.activityControllerFake.Inspect(ctx, id, mode)
	if err == nil && id == "h-c" && handle != nil && handle.Status == domain.HandleCompleted {
		f.completionMu.Lock()
		f.producerCompleted = true
		f.completionMu.Unlock()
	}
	return handle, err
}
