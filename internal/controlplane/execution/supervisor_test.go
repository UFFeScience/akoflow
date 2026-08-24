package execution

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type executionStoreFake struct {
	runs    []domain.ExecutionRun
	tasks   []domain.TaskExecution
	trace   domain.ExecutionTrace
	handles map[string]domain.ActivityHandle
	failed  string
}

func (f *executionStoreFake) CreateRun(_ context.Context, run domain.ExecutionRun) error {
	f.runs = append(f.runs, run)
	return nil
}
func (f *executionStoreFake) FindRun(context.Context, string) (*domain.ExecutionRun, error) {
	return nil, nil
}
func (f *executionStoreFake) SaveTask(_ context.Context, task domain.TaskExecution) error {
	f.tasks = append(f.tasks, task)
	return nil
}
func (f *executionStoreFake) CompleteRun(_ context.Context, trace domain.ExecutionTrace) error {
	f.trace = trace
	return nil
}
func (f *executionStoreFake) FailRun(_ context.Context, _ string, reason string) error {
	f.failed = reason
	return nil
}
func (f *executionStoreFake) Save(_ context.Context, h domain.ActivityHandle) error {
	if f.handles == nil {
		f.handles = map[string]domain.ActivityHandle{}
	}
	f.handles[h.ID] = h
	return nil
}
func (f *executionStoreFake) Find(_ context.Context, id string) (*domain.ActivityHandle, error) {
	h, ok := f.handles[id]
	if !ok {
		return nil, nil
	}
	return &h, nil
}
func (f *executionStoreFake) ListHandles(_ context.Context, runID string) ([]domain.ActivityHandle, error) {
	result := make([]domain.ActivityHandle, 0, len(f.handles))
	for _, handle := range f.handles {
		if handle.RunID == runID {
			result = append(result, handle)
		}
	}
	return result, nil
}

type activityControllerFake struct {
	started     []string
	inspections map[string]int
	startErr    error
}

func (f *activityControllerFake) Start(_ context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	f.started = append(f.started, execution.Activity.ID)
	if f.startErr != nil {
		return domain.ActivityHandle{}, f.startErr
	}
	return domain.ActivityHandle{
		ID: "h-" + execution.Activity.ID, RunID: execution.Run.ID,
		ActivityID: execution.Activity.ID, ResourceID: execution.Resource.ID,
		RuntimeID: execution.RuntimeID, Status: domain.HandleRunning, StartedAt: 1,
	}, nil
}
func (f *activityControllerFake) Inspect(_ context.Context, id string, _ domain.ExecutionMode) (*domain.ActivityHandle, error) {
	if f.inspections == nil {
		f.inspections = map[string]int{}
	}
	f.inspections[id]++
	return &domain.ActivityHandle{ID: id, Status: domain.HandleCompleted, StartedAt: 1, FinishedAt: 2}, nil
}

type planExecutorFake struct{ called bool }

func (f *planExecutorFake) Execute(_ context.Context, request ports.ExecutionRequest) (domain.ExecutionTrace, error) {
	f.called = true
	return domain.ExecutionTrace{RunID: request.Run.ID, PlanID: request.Plan.ID, Mode: request.Run.Mode}, nil
}

func requestFixture(mode domain.ExecutionMode) ports.ExecutionRequest {
	simulation := &domain.ActivitySimulation{DurationSeconds: 1}
	activity := func(id string) domain.Activity {
		return domain.Activity{
			ID: id, Name: id, Kind: domain.ActivityKindTask,
			Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal},
			Command:      domain.ActivityCommand{Entrypoint: "true"}, Simulation: simulation,
		}
	}
	runtimeMode := domain.RuntimeModeExecution
	runtimeDriver := domain.RuntimeDriverLocal
	runtimeID := "local"
	if mode == domain.ExecutionModeSimulation {
		runtimeMode = domain.RuntimeModeSimulation
		runtimeDriver = domain.RuntimeDriverSimGrid
		runtimeID = "simgrid"
	}
	return ports.ExecutionRequest{
		Run: domain.ExecutionRun{ID: "run", Mode: mode},
		Plan: domain.SchedulePlan{
			ID: "plan",
			Assignments: []domain.PlanAssignment{
				{ID: "pa", ActivityID: "a", ResourceID: "r"},
				{ID: "pb", ActivityID: "b", ResourceID: "r"},
			},
		},
		Workflow: domain.WorkflowVersion{
			ID: "workflow", Activities: []domain.Activity{activity("a"), activity("b")},
			Dependencies: []domain.ActivityDependency{{
				ActivityID: "b", DependsOnActivityID: "a",
			}},
		},
		Resources:       []domain.Resource{{ID: "r"}},
		Runtimes:        []domain.EnvironmentRuntime{{ID: runtimeID, Name: runtimeID, Driver: runtimeDriver, Mode: runtimeMode}},
		RuntimeBindings: []domain.ResourceRuntimeBinding{{ResourceID: "r", RuntimeID: runtimeID, Enabled: true}},
	}
}

func TestSelectRuntimeHonorsPlannedRuntime(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	request.Runtimes = append(request.Runtimes, domain.EnvironmentRuntime{ID: "ssh", Name: "ssh", Driver: domain.RuntimeDriverSSH, Mode: domain.RuntimeModeExecution})
	request.RuntimeBindings = append(request.RuntimeBindings, domain.ResourceRuntimeBinding{ResourceID: "r", RuntimeID: "ssh", Enabled: true})
	assignment := request.Plan.Assignments[0]
	assignment.Metadata = map[string]any{"runtimeId": "ssh", "partitionId": "short"}
	if got := selectRuntime(request, assignment); got != "ssh" {
		t.Fatalf("runtime=%q, want ssh", got)
	}
	assignment.Metadata["runtimeId"] = "missing"
	if got := selectRuntime(request, assignment); got != "" {
		t.Fatalf("runtime=%q, want empty for unavailable planned runtime", got)
	}
}

func TestSupervisorExecutesDAGInDependencyOrder(t *testing.T) {
	store := &executionStoreFake{}
	activities := &activityControllerFake{}
	simulator := &planExecutorFake{}
	service, err := New(store, activities, simulator, Config{PollInterval: time.Microsecond, MaxParallel: 2})
	if err != nil {
		t.Fatal(err)
	}
	trace, err := service.Execute(context.Background(), requestFixture(domain.ExecutionModeReal))
	if err != nil {
		t.Fatal(err)
	}
	if len(activities.started) != 2 || activities.started[0] != "a" || activities.started[1] != "b" {
		t.Fatalf("order=%v", activities.started)
	}
	if trace.RunID != "run" || store.trace.RunID != "run" {
		t.Fatal("trace was not completed")
	}
}

func TestSupervisorDelegatesSimulation(t *testing.T) {
	store := &executionStoreFake{}
	simulator := &planExecutorFake{}
	service, _ := New(store, &activityControllerFake{}, simulator, Config{})
	request := requestFixture(domain.ExecutionModeSimulation)
	if _, err := service.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if !simulator.called {
		t.Fatal("simulator was not called")
	}
}

func TestSupervisorRejectsIncompletePlan(t *testing.T) {
	service, _ := New(&executionStoreFake{}, &activityControllerFake{}, &planExecutorFake{}, Config{})
	request := requestFixture(domain.ExecutionModeReal)
	request.Plan.Assignments = nil
	if _, err := service.Execute(context.Background(), request); err == nil {
		t.Fatal("incomplete plan must fail")
	}
}

func TestCompletedTaskSeparatesContainerOverheadFromCompute(t *testing.T) {
	task := domain.TaskExecution{}
	completeTask(&task, domain.ActivityHandle{
		StartedAt: 10, FinishedAt: 18,
		Metadata: map[string]any{
			domain.TimingSubmittedAt:        7.0,
			domain.TimingContainerStartedAt: 12.5,
		},
	})
	if task.QueueSeconds != 3 || task.OverheadSeconds != 2.5 || task.RuntimeSeconds != 5.5 {
		t.Fatalf("timing=%+v", task)
	}
}

func TestSupervisorMarksActivityFailedWhenStartIsRejected(t *testing.T) {
	store := &executionStoreFake{}
	activities := &activityControllerFake{startErr: fmt.Errorf("activity image is required for Kubernetes")}
	service, _ := New(store, activities, &planExecutorFake{}, Config{})

	if _, err := service.Execute(context.Background(), requestFixture(domain.ExecutionModeReal)); err == nil {
		t.Fatal("execution must fail when the activity cannot start")
	}
	if store.failed == "" {
		t.Fatal("run was not marked failed")
	}
	if len(store.tasks) != 1 || store.tasks[0].Status != domain.TaskFailed {
		t.Fatalf("tasks=%+v, want one failed task", store.tasks)
	}
	if store.tasks[0].FailureReason != "start activity: activity image is required for Kubernetes" {
		t.Fatalf("failure=%q", store.tasks[0].FailureReason)
	}
	handle := store.handles["run:a"]
	if handle.Status != domain.HandleFailed {
		t.Fatalf("handle status=%q, want failed", handle.Status)
	}
}
