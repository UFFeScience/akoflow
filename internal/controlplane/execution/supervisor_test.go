package execution

import (
	"context"
	"fmt"
	"strings"
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
	contexts    []domain.ActivityExecutionContext
	inspections map[string]int
	startErr    error
}

func (f *activityControllerFake) Start(_ context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	f.started = append(f.started, execution.Activity.ID)
	f.contexts = append(f.contexts, execution)
	if f.startErr != nil {
		return domain.ActivityHandle{}, f.startErr
	}
	return domain.ActivityHandle{
		ID: "h-" + execution.Activity.ID, RunID: execution.Run.ID,
		ActivityID: execution.Activity.ID, ResourceID: execution.Resource.ID,
		RuntimeID: execution.RuntimeID, Status: domain.HandleRunning, StartedAt: 1,
	}, nil
}

type cloudAllocationStoreFake struct {
	ports.CloudConfigurationStore
	target    domain.CloudCapacityTarget
	instances []domain.CloudProvisionedInstance
}

func (f cloudAllocationStoreFake) FindCapacityTarget(context.Context, string) (*domain.CloudCapacityTarget, error) {
	value := f.target
	return &value, nil
}

func (f cloudAllocationStoreFake) ListProvisionedInstances(context.Context, string) ([]domain.CloudProvisionedInstance, error) {
	return f.instances, nil
}

type cloudProvisionerFake struct {
	ports.CloudProvisioner
	instance domain.CloudProvisionedInstance
	calls    int
}

func (f *cloudProvisionerFake) Provision(context.Context, string, domain.CloudProvisionRequest) (domain.CloudProvisionedInstance, error) {
	f.calls++
	return f.instance, nil
}

func (*cloudProvisionerFake) Release(context.Context, []string) error { return nil }
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

func TestExpectedOutputsRequireObservedChecksummedFiles(t *testing.T) {
	instances := []domain.DataObjectInstance{
		{ProducerActivityID: "producer", RelativePath: "result.fits", Checksum: "sha256:valid"},
		{ProducerActivityID: "other", RelativePath: "missing.fits", Checksum: "sha256:other"},
	}
	if err := validateExpectedOutputInstances("producer", []string{"result.fits"}, instances); err != nil {
		t.Fatal(err)
	}
	if err := validateExpectedOutputInstances("producer", []string{"missing.fits"}, instances); err == nil {
		t.Fatal("a required output on another resource must not satisfy this producer")
	}
}

func TestFailedRunRetainsCloudInstanceHoldingEphemeralOutputs(t *testing.T) {
	allocations := map[string]domain.RuntimeAllocation{
		"producer": {ResourceID: "cloud-a", CloudInstanceID: "vm-a"},
		"sibling":  {ResourceID: "cloud-a", CloudInstanceID: "vm-a"},
		"other":    {ResourceID: "cloud-b", CloudInstanceID: "vm-b"},
	}
	remaining, retained := retainCloudSourcesWithEphemeralOutputs(allocations, []domain.DataLocation{
		{ResourceID: "cloud-a", Status: domain.DataLocationEphemeral},
	})
	if retained != 1 || len(remaining) != 1 || remaining["other"].CloudInstanceID != "vm-b" {
		t.Fatalf("remaining=%+v retained=%d", remaining, retained)
	}
}

func TestWorkspaceLocationsFollowAssignedRuntime(t *testing.T) {
	tests := []struct {
		name          string
		driver        domain.RuntimeDriver
		configuration map[string]any
		metadata      map[string]any
		wantSource    string
		wantTarget    string
	}{
		{
			name: "local", driver: domain.RuntimeDriverLocal,
			wantSource: localWorkspaceURI("run", "producer"),
			wantTarget: localWorkspaceURI("run", "producer"),
		},
		{
			name: "kubernetes", driver: domain.RuntimeDriverKubernetes,
			configuration: map[string]any{"connectionId": "cluster", "namespace": "science"},
			wantSource:    "kubernetes://cluster/tmp/akoflow/workspace?claim=akoflow-run-producer-workspace&namespace=science",
			wantTarget:    "activityId=producer&claim=akoflow-run-producer-workspace&claimBytes=24&createClaim=true&namespace=science&runId=run",
		},
		{
			name: "slurm", driver: domain.RuntimeDriverSlurm,
			configuration: map[string]any{"connectionId": "hpc"},
			metadata:      map[string]any{"homeDirectory": "/home/scientist"},
			wantSource:    "file:///home/scientist/akoflow-workspaces/run/producer?connectionId=hpc",
			wantTarget:    "file:///home/scientist/akoflow-workspaces/run/producer?connectionId=hpc",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := ports.ExecutionRequest{
				Run: domain.ExecutionRun{ID: "run"},
				Plan: domain.SchedulePlan{Assignments: []domain.PlanAssignment{{
					ActivityID: "producer", ResourceID: "resource",
				}}},
				Resources: []domain.Resource{{ID: "resource", EnvironmentVersionID: "environment", Metadata: test.metadata}},
				Runtimes: []domain.EnvironmentRuntime{{
					ID: "runtime", Driver: test.driver, Mode: domain.RuntimeModeExecution, Configuration: test.configuration,
				}},
				RuntimeBindings: []domain.ResourceRuntimeBinding{{
					ResourceID: "resource", RuntimeID: "runtime", Enabled: true,
				}},
			}
			source, err := workspaceSourceForActivity(request, "producer")
			if err != nil {
				t.Fatal(err)
			}
			if source.URI != test.wantSource {
				t.Fatalf("source=%q, want %q", source.URI, test.wantSource)
			}
			destination, err := workspaceDestination(request, "producer", request.Resources[0], 12)
			if err != nil {
				t.Fatal(err)
			}
			if test.driver == domain.RuntimeDriverKubernetes {
				if !strings.Contains(destination.URI, test.wantTarget) {
					t.Fatalf("destination=%q, want query %q", destination.URI, test.wantTarget)
				}
			} else if destination.URI != test.wantTarget {
				t.Fatalf("destination=%q, want %q", destination.URI, test.wantTarget)
			}
		})
	}
}

func TestWorkspaceClaimNameIsKubernetesSafeAndBounded(t *testing.T) {
	name := workspaceClaimName("RUN_With.Invalid/Characters", strings.Repeat("activity", 20))
	if len(name) > 63 {
		t.Fatalf("claim name has %d characters", len(name))
	}
	if strings.ContainsAny(name, "_./ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		t.Fatalf("claim name is not normalized: %q", name)
	}
}

func TestTransferObservationsPreserveDirectWorkspaceRoute(t *testing.T) {
	requirement := domain.PreparationRequirement{WorkspaceTransfers: []domain.DataTransferPlan{{
		ID: "workspace-plan", ProducerActivityID: "k6",
		Source: domain.TransferLocation{ResourceID: "source"},
	}}}
	topology := domain.NetworkTopology{Links: []domain.NetworkLink{{
		SourceResourceID: "source", TargetResourceID: "target", PricePerByte: 0.5,
	}}}
	transfers := transferObservations("run", "k7", "target", []string{"k6"}, requirement, []domain.DataTransferRun{{
		ID: "transfer", PlanID: "workspace-plan", TransferredBytes: 42, FilesTransferred: 2, StartedAt: 5, FinishedAt: 8,
	}}, topology)
	if len(transfers) != 1 {
		t.Fatalf("transfers=%+v", transfers)
	}
	got := transfers[0]
	if got.ProducerActivityID != "k6" || got.ConsumerActivityID != "k7" || got.SourceResourceID != "source" || got.TargetResourceID != "target" || got.DurationSeconds != 3 || got.Bytes != 42 || got.FilesTransferred != 2 || got.Cost != 21 {
		t.Fatalf("unexpected transfer observation: %+v", got)
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

func TestSupervisorBindsCloudInstanceBeforeActivityStart(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	request.Workflow.Activities = request.Workflow.Activities[:1]
	request.Workflow.Dependencies = nil
	request.Plan.Assignments = request.Plan.Assignments[:1]
	request.Plan.Assignments[0].Metadata = map[string]any{"runtimeId": "cloud-runtime"}
	request.Resources[0].Type = domain.ResourceCloudVM
	request.Resources[0].Metadata = map[string]any{"capacityTargetId": "capacity"}
	request.Runtimes = []domain.EnvironmentRuntime{{
		ID: "cloud-runtime", Driver: domain.RuntimeDriverCloud, Mode: domain.RuntimeModeExecution,
		Configuration: map[string]any{"connectionId": "cloud-connection"},
	}}
	request.RuntimeBindings = []domain.ResourceRuntimeBinding{{ResourceID: "r", RuntimeID: "cloud-runtime", Enabled: true}}
	cloud := &cloudProvisionerFake{instance: domain.CloudProvisionedInstance{
		ID: "instance-a", CapacityTargetID: "capacity", EnvironmentID: "environment", Status: "ready",
	}}
	store := &executionStoreFake{}
	activities := &activityControllerFake{}
	service, err := New(store, activities, &planExecutorFake{}, Config{
		PollInterval: time.Microsecond, MaxParallel: 1, Cloud: cloud,
		CloudStore: cloudAllocationStoreFake{target: domain.CloudCapacityTarget{ID: "capacity", EnvironmentID: "environment"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if cloud.calls != 1 || len(activities.contexts) != 1 {
		t.Fatalf("provision calls=%d contexts=%d", cloud.calls, len(activities.contexts))
	}
	allocation := activities.contexts[0].Allocation
	if allocation.CloudInstanceID != "instance-a" || allocation.RuntimeID != "cloud-runtime" || allocation.ConnectionID != "cloud-connection" {
		t.Fatalf("unexpected concrete allocation: %#v", allocation)
	}
	if len(store.tasks) == 0 || store.tasks[0].CloudInstanceID != "instance-a" {
		t.Fatalf("task did not persist allocation: %#v", store.tasks)
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

func TestCompletedTaskAccountsForObservedRuntimeCost(t *testing.T) {
	task := domain.TaskExecution{
		Metadata: map[string]any{"pricePerSecond": 0.25},
	}
	completeTask(&task, domain.ActivityHandle{StartedAt: 10, FinishedAt: 14})
	if task.RuntimeSeconds != 4 || task.Cost != 1 {
		t.Fatalf("task=%#v", task)
	}
}

func TestCompletedTraceIncludesObservedTransferCost(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	task := domain.TaskExecution{
		ActivityID: "a",
		StartedAt:  10,
		FinishedAt: 14,
		Cost:       1,
	}
	trace := completedTrace(request, map[string]domain.TaskExecution{"a": task}, []domain.DataTransfer{{Cost: 2.5}})
	if trace.Executed.Cost != 3.5 {
		t.Fatalf("cost=%v", trace.Executed.Cost)
	}
}

func TestObservedCloudCostIncludesIdleWindowAndPersistentDisk(t *testing.T) {
	tasks := []domain.TaskExecution{
		{CloudInstanceID: "instance-a", AllocatedResourceID: "cloud-a", StartedAt: 10, FinishedAt: 12, Cost: 2},
		{CloudInstanceID: "instance-a", AllocatedResourceID: "cloud-a", StartedAt: 15, FinishedAt: 17, Cost: 2},
	}
	resources := []domain.Resource{{
		ID:             "cloud-a",
		PricePerSecond: 1,
		StorageBytes:   10 << 30,
		Metadata: map[string]any{
			"diskPricePerGiBMonth": float64(730 * 3600 / 10),
		},
	}}

	// Seven seconds of allocated compute cost seven units. The activities
	// already account for four, leaving three idle units. The 10 GiB disk at
	// the test rate contributes another seven units over the same window.
	if cost := observedCloudIdleAndDiskCost(tasks, resources); cost != 10 {
		t.Fatalf("cloud idle and disk cost=%v, want 10", cost)
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
