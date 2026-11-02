package execution

import (
	"context"
	"errors"
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestCancelledInspectionDoesNotPersistComputationFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runtime := &runtimeFake{inspectErr: context.Canceled}
	handles := &handlesFake{handle: &domain.ActivityHandle{ID: "h", RuntimeID: "runtime", Status: domain.HandleRunning}}
	h, err := New(resolverFake{adapter: runtime}, handles).Inspect(ctx, "h", domain.ExecutionModeReal)
	if !errors.Is(err, context.Canceled) || h.Status != domain.HandleRunning || handles.handle.Status != domain.HandleRunning {
		t.Fatalf("handle=%+v err=%v", h, err)
	}
}

type stoppingRuntime struct {
	runtimeFake
	entered     chan struct{}
	release     chan struct{}
	inspections int
}

func (r *stoppingRuntime) Stop(context.Context, domain.ActivityHandle) error {
	close(r.entered)
	<-r.release
	return nil
}
func (r *stoppingRuntime) Inspect(context.Context, domain.ActivityHandle) (domain.ActivityHandle, error) {
	r.inspections++
	return domain.ActivityHandle{}, nil
}

func TestInspectDoesNotQueryRemovedRuntimeDuringStop(t *testing.T) {
	runtime := &stoppingRuntime{entered: make(chan struct{}), release: make(chan struct{})}
	handles := &handlesFake{handle: &domain.ActivityHandle{ID: "h", RuntimeID: "runtime", Status: domain.HandleRunning}}
	controller := New(resolverFake{adapter: runtime}, handles)
	stopped := make(chan error, 1)
	go func() { stopped <- controller.Stop(context.Background(), "h", domain.ExecutionModeReal) }()
	<-runtime.entered
	inspected := make(chan *domain.ActivityHandle, 1)
	go func() {
		h, err := controller.Inspect(context.Background(), "h", domain.ExecutionModeReal)
		if err != nil {
			inspected <- nil
			return
		}
		inspected <- h
	}()
	close(runtime.release)
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	h := <-inspected
	if h == nil || h.Status != domain.HandleStopped || runtime.inspections != 0 {
		t.Fatalf("handle=%+v runtime inspections=%d", h, runtime.inspections)
	}
}
