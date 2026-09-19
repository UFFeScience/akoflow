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
	startErr   error
	inspectErr error
	stopErr    error
}

func (f *runtimeFake) Modes() []domain.ExecutionMode {
	return []domain.ExecutionMode{domain.ExecutionModeReal}
}
func (f *runtimeFake) Start(context.Context, domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	f.starts++
	return f.handle, f.startErr
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
	return f.stopErr
}

type handlesFake struct {
	handle  *domain.ActivityHandle
	metrics []domain.ActivityMetricSample
	findErr error
	saveErr error
}

type catalogFake struct {
	ports.DataCatalog
	err   error
	calls int
}

func (f *catalogFake) CatalogArtifacts(context.Context, domain.ActivityHandle) error {
	f.calls++
	return f.err
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
func (f *handlesFake) SaveActivityMetrics(_ context.Context, metrics []domain.ActivityMetricSample) error {
	f.metrics = append(f.metrics, metrics...)
	return nil
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

func TestStartPersistsInitialRuntimeMetrics(t *testing.T) {
	runtime := &runtimeFake{handle: domain.ActivityHandle{
		RunID: "run", ActivityID: "activity", RuntimeID: "local", Status: domain.HandleRunning,
		Metrics: []domain.ActivityMetricSample{{RunID: "run", ActivityID: "activity", Attempt: 1, ObservedAt: 10}},
	}}
	handles := &handlesFake{}
	if _, err := New(resolverFake{adapter: runtime}, handles).Start(context.Background(), executionFixture()); err != nil {
		t.Fatal(err)
	}
	if len(handles.metrics) != 1 || handles.metrics[0].ObservedAt != 10 {
		t.Fatalf("initial metrics were not persisted: %+v", handles.metrics)
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

func TestStartAppliesCommittedPreparation(t *testing.T) {
	runtime := &runtimeFake{handle: domain.ActivityHandle{RunID: "run", ActivityID: "activity", RuntimeID: "local", Status: domain.HandleCompleted}}
	execution := executionFixture()
	execution.Preparation = &domain.PreparationGate{
		Executable: &domain.ArtifactMaterialization{ID: "materialization", VariantID: "variant", Digest: "sha256:digest", DestinationPath: "/work/bin/tool", EnvironmentID: "env", ResourceID: "node", Status: domain.MaterializationCommitted, VerifiedDigest: "sha256:digest"},
		Workspace:  &domain.WorkspaceMaterialization{Destination: domain.TransferLocation{URI: "file:///work/run"}, Status: domain.MaterializationCommitted},
	}

	handle, err := New(resolverFake{adapter: runtime}, &handlesFake{}).Start(context.Background(), execution)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(handle.Log, "Activity started") || !strings.Contains(handle.Log, "completed successfully") {
		t.Fatalf("missing lifecycle logs: %q", handle.Log)
	}
}

func TestStartRejectsInvalidInputsAndRuntimeResults(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*domain.ActivityExecutionContext, *runtimeFake, *handlesFake) resolverFake
		want      string
	}{
		{name: "invalid activity", configure: func(e *domain.ActivityExecutionContext, _ *runtimeFake, _ *handlesFake) resolverFake {
			e.Activity.ID = ""
			return resolverFake{}
		}, want: "activity"},
		{name: "unready preparation", configure: func(e *domain.ActivityExecutionContext, _ *runtimeFake, _ *handlesFake) resolverFake {
			e.Preparation = &domain.PreparationGate{Executable: &domain.ArtifactMaterialization{Status: domain.MaterializationPlanned}}
			return resolverFake{}
		}, want: "preparation"},
		{name: "unknown mode", configure: func(e *domain.ActivityExecutionContext, _ *runtimeFake, _ *handlesFake) resolverFake {
			e.Run.Mode = domain.ExecutionMode("unknown")
			return resolverFake{}
		}, want: "unsupported execution mode"},
		{name: "find error", configure: func(_ *domain.ActivityExecutionContext, _ *runtimeFake, h *handlesFake) resolverFake {
			h.findErr = errors.New("lookup")
			return resolverFake{}
		}, want: "find activity handle"},
		{name: "missing runtime", configure: func(e *domain.ActivityExecutionContext, _ *runtimeFake, _ *handlesFake) resolverFake {
			e.RuntimeID = ""
			return resolverFake{}
		}, want: "no selected runtime"},
		{name: "resolve error", configure: func(_ *domain.ActivityExecutionContext, _ *runtimeFake, _ *handlesFake) resolverFake {
			return resolverFake{err: errors.New("resolve")}
		}, want: "resolve"},
		{name: "start error", configure: func(_ *domain.ActivityExecutionContext, r *runtimeFake, _ *handlesFake) resolverFake {
			r.startErr = errors.New("start")
			return resolverFake{adapter: r}
		}, want: "start"},
		{name: "invalid handle", configure: func(_ *domain.ActivityExecutionContext, r *runtimeFake, _ *handlesFake) resolverFake {
			r.handle = domain.ActivityHandle{RunID: "other", ActivityID: "activity"}
			return resolverFake{adapter: r}
		}, want: "invalid activity handle"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			execution, runtime, handles := executionFixture(), &runtimeFake{}, &handlesFake{}
			resolver := test.configure(&execution, runtime, handles)
			_, err := New(resolver, handles).Start(context.Background(), execution)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want containing %q", err, test.want)
			}
		})
	}
}

func TestStartLogsFailedHandleWithoutFailureReason(t *testing.T) {
	runtime := &runtimeFake{handle: domain.ActivityHandle{RunID: "run", ActivityID: "activity", RuntimeID: "local", Status: domain.HandleFailed}}
	handle, err := New(resolverFake{adapter: runtime}, &handlesFake{}).Start(context.Background(), executionFixture())
	if err != nil || !strings.Contains(handle.Log, "stopped without a reported failure reason") {
		t.Fatalf("handle=%+v err=%v", handle, err)
	}
}

func TestInspectCatalogsTerminalArtifactsAndHandlesCatalogFailure(t *testing.T) {
	for _, status := range []domain.ActivityHandleStatus{domain.HandleCompleted, domain.HandleFailed} {
		t.Run(string(status), func(t *testing.T) {
			runtime := &runtimeFake{handle: domain.ActivityHandle{ID: "handle", RuntimeID: "local", Status: status, Artifacts: &domain.ArtifactManifest{}}}
			handles := &handlesFake{handle: &domain.ActivityHandle{ID: "handle", RuntimeID: "local", Status: domain.HandleRunning}}
			catalog := &catalogFake{err: errors.New("catalog unavailable")}
			updated, err := New(resolverFake{adapter: runtime}, handles, catalog).Inspect(context.Background(), "handle", domain.ExecutionModeReal)
			if err != nil || catalog.calls != 1 || updated.Metadata["artifactCatalogError"] == nil {
				t.Fatalf("updated=%+v calls=%d err=%v", updated, catalog.calls, err)
			}
			if status == domain.HandleCompleted && updated.Status != domain.HandleFailed {
				t.Fatalf("completed handle must fail when outputs cannot be cataloged: %+v", updated)
			}
		})
	}
}

func TestInspectAndStopFailures(t *testing.T) {
	service := New(resolverFake{}, &handlesFake{findErr: errors.New("find")})
	if _, err := service.Inspect(context.Background(), "missing", domain.ExecutionModeReal); err == nil {
		t.Fatal("inspect find error expected")
	}
	if err := service.Stop(context.Background(), "missing", domain.ExecutionModeReal); err == nil {
		t.Fatal("stop find error expected")
	}
	if err := New(resolverFake{}, &handlesFake{}).Stop(context.Background(), "missing", domain.ExecutionModeReal); err == nil {
		t.Fatal("missing handle error expected")
	}

	handle := domain.ActivityHandle{ID: "handle", RuntimeID: "runtime", Status: domain.HandleRunning}
	if err := New(resolverFake{err: errors.New("resolve")}, &handlesFake{handle: &handle}).Stop(context.Background(), "handle", domain.ExecutionModeReal); err == nil {
		t.Fatal("resolve error expected")
	}
	runtime := &runtimeFake{stopErr: errors.New("stop")}
	if err := New(resolverFake{adapter: runtime}, &handlesFake{handle: &handle}).Stop(context.Background(), "handle", domain.ExecutionModeReal); err == nil {
		t.Fatal("runtime stop error expected")
	}
	runtime.stopErr = nil
	if err := New(resolverFake{adapter: runtime}, &handlesFake{handle: &handle, saveErr: errors.New("save")}).Stop(context.Background(), "handle", domain.ExecutionModeReal); err == nil {
		t.Fatal("save error expected")
	}
}

func TestInspectFailureReportsPersistenceError(t *testing.T) {
	handle := domain.ActivityHandle{ID: "handle", RuntimeID: "runtime", Status: domain.HandleRunning}
	_, err := New(resolverFake{err: errors.New("resolve")}, &handlesFake{handle: &handle, saveErr: errors.New("save")}).Inspect(context.Background(), "handle", domain.ExecutionModeReal)
	if err == nil || !strings.Contains(err.Error(), "save failed handle") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLifecycleLogHelpers(t *testing.T) {
	log := appendLog("runtime output\n", "info", "started")
	if duplicate := appendLog(log, "info", "started"); duplicate != log {
		t.Fatalf("duplicate lifecycle entry added: %q", duplicate)
	}
	if got := appendLog(log, "info", ""); got != log {
		t.Fatalf("empty message changed log: %q", got)
	}
	if got := mergeLogs(log, "runtime output\nmore output\n"); !strings.Contains(got, "more output") || !strings.Contains(got, "AKOFLOW") {
		t.Fatalf("logs were not merged: %q", got)
	}
	if got := mergeLogs("runtime output\nmore\n", "runtime output\n"); got != "runtime output\nmore\n" {
		t.Fatalf("short observation replaced complete log: %q", got)
	}
	if got := mergeLogs("old\n[AKOFLOW INFO] started\n", "new\n"); !strings.Contains(got, "new") || !strings.Contains(got, "started") {
		t.Fatalf("lifecycle log not preserved: %q", got)
	}
}
