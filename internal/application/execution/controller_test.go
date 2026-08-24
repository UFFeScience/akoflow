package execution

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type resolverFake struct {
	adapter ports.RuntimeAdapter
	err     error
}

func (f resolverFake) Resolve(domain.ExecutionMode, string) (ports.RuntimeAdapter, error) {
	return f.adapter, f.err
}

type runtimeFake struct {
	handle     domain.ActivityHandle
	stopped    bool
	starts     int
	inspectErr error
}

func (f *runtimeFake) Modes() []domain.ExecutionMode {
	return []domain.ExecutionMode{domain.ExecutionModeReal}
}
func (f *runtimeFake) Start(context.Context, domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	f.starts++
	return f.handle, nil
}
func (f *runtimeFake) Inspect(context.Context, domain.ActivityHandle) (domain.ActivityHandle, error) {
	if f.inspectErr != nil {
		return domain.ActivityHandle{}, f.inspectErr
	}
	f.handle.Status = domain.HandleCompleted
	return f.handle, nil
}
func (f *runtimeFake) Stop(context.Context, domain.ActivityHandle) error {
	f.stopped = true
	return nil
}

type handlesFake struct {
	handle  *domain.ActivityHandle
	findErr error
	saveErr error
}

func (f *handlesFake) Save(_ context.Context, handle domain.ActivityHandle) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.handle = &handle
	return nil
}
func (f *handlesFake) Find(context.Context, string) (*domain.ActivityHandle, error) {
	return f.handle, f.findErr
}

func executionFixture() domain.ActivityExecutionContext {
	return domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run", Mode: domain.ExecutionModeReal},
		Activity: domain.Activity{ID: "activity", Name: "activity", Kind: domain.ActivityKindTask,
			Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal},
			Command:      domain.ActivityCommand{Entrypoint: "echo"}},
		Resource: domain.Resource{ID: "node"}, RuntimeID: "local",
	}
}

func TestStartPersistsRuntimeIndependentHandle(t *testing.T) {
	runtime := &runtimeFake{handle: domain.ActivityHandle{ID: "handle", RunID: "run", ActivityID: "activity", RuntimeID: "local", Status: domain.HandleRunning}}
	handles := &handlesFake{}
	got, err := New(resolverFake{adapter: runtime}, handles).Start(context.Background(), executionFixture())
	if err != nil || got.ID != "run:activity" || handles.handle == nil {
		t.Fatalf("unexpected start: %+v %v", got, err)
	}
}

func TestStartReturnsPersistedHandleWithoutStartingRuntimeAgain(t *testing.T) {
	runtime := &runtimeFake{}
	existing := domain.ActivityHandle{ID: "run:activity", RunID: "run", ActivityID: "activity", Status: domain.HandleRunning}
	got, err := New(resolverFake{adapter: runtime}, &handlesFake{handle: &existing}).Start(context.Background(), executionFixture())
	if err != nil || got.ID != existing.ID || runtime.starts != 0 {
		t.Fatalf("handle=%+v err=%v", got, err)
	}
}

func TestStartStopsRuntimeWhenHandleCannotBePersisted(t *testing.T) {
	runtime := &runtimeFake{handle: domain.ActivityHandle{ID: "handle", RunID: "run", ActivityID: "activity"}}
	_, err := New(resolverFake{adapter: runtime}, &handlesFake{saveErr: errors.New("database")}).Start(context.Background(), executionFixture())
	if err == nil || !runtime.stopped {
		t.Fatal("runtime must be stopped after persistence failure")
	}
}

func TestStartRejectsUnsupportedMode(t *testing.T) {
	execution := executionFixture()
	execution.Run.Mode = domain.ExecutionModeInteractive
	_, err := New(resolverFake{}, &handlesFake{}).Start(context.Background(), execution)
	if err == nil {
		t.Fatal("unsupported activity mode must fail")
	}
}

func TestInspectAndStop(t *testing.T) {
	runtime := &runtimeFake{handle: domain.ActivityHandle{ID: "handle", RuntimeID: "local", Status: domain.HandleRunning}}
	handles := &handlesFake{handle: &runtime.handle}
	service := New(resolverFake{adapter: runtime}, handles)
	updated, err := service.Inspect(context.Background(), "handle", domain.ExecutionModeReal)
	if err != nil || updated.Status != domain.HandleCompleted {
		t.Fatal("inspect failed")
	}
	if err := service.Stop(context.Background(), "handle", domain.ExecutionModeReal); err != nil || !runtime.stopped || handles.handle.Status != domain.HandleStopped {
		t.Fatal("stop failed")
	}
}

func TestInspectPersistsFailureLogForAnyRuntimeError(t *testing.T) {
	runtime := &runtimeFake{inspectErr: errors.New("runtime API unavailable")}
	handle := domain.ActivityHandle{ID: "handle", RunID: "run", ActivityID: "activity", RuntimeID: "runtime", Status: domain.HandleRunning}
	handles := &handlesFake{handle: &handle}
	updated, err := New(resolverFake{adapter: runtime}, handles).Inspect(context.Background(), "handle", domain.ExecutionModeReal)
	if err != nil || updated.Status != domain.HandleFailed {
		t.Fatalf("inspect failure was not persisted: handle=%+v err=%v", updated, err)
	}
	if !strings.Contains(updated.Log, "runtime API unavailable") || !strings.Contains(updated.Log, "[AKOFLOW ERROR]") {
		t.Fatalf("missing runtime failure log: %q", updated.Log)
	}
}
