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
	started                    []string
	contexts                   []domain.ActivityExecutionContext
	inspections                map[string]int
	startErr                   error
	startEvents                chan string
	minimumStartsBeforeInspect int
	firstInspectStarts         int
}

func (f *activityControllerFake) Start(_ context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	f.started = append(f.started, execution.Activity.ID)
	f.contexts = append(f.contexts, execution)
	if f.startEvents != nil {
		f.startEvents <- execution.Activity.ID
	}
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
	targets   map[string]domain.CloudCapacityTarget
	instances []domain.CloudProvisionedInstance
}

func (f cloudAllocationStoreFake) FindCapacityTarget(_ context.Context, id string) (*domain.CloudCapacityTarget, error) {
	if f.targets != nil {
		value, ok := f.targets[id]
		if !ok {
			return nil, nil
		}
		return &value, nil
	}
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

type parallelPrewarmFake struct {
	prewarmed []string
	want      int
}

type staggeredCloudAllocatorFake struct {
	slowReady <-chan struct{}
}

func (f *staggeredCloudAllocatorFake) Prewarm(context.Context, string, string, domain.CloudCapacityTarget) error {
	return nil
}

func (f *staggeredCloudAllocatorFake) Allocate(ctx context.Context, _ string, _ string, target domain.CloudCapacityTarget) (domain.CloudProvisionedInstance, error) {
	if target.ID == "target-0" {
		select {
		case <-f.slowReady:
		case <-ctx.Done():
			return domain.CloudProvisionedInstance{}, ctx.Err()
		}
	}
	return domain.CloudProvisionedInstance{ID: "instance-" + target.ID, CapacityTargetID: target.ID, EnvironmentID: target.EnvironmentID, Status: "ready"}, nil
}

func (*staggeredCloudAllocatorFake) Release(context.Context, string, map[string]domain.RuntimeAllocation, bool) error {
	return nil
}

func (f *parallelPrewarmFake) Prewarm(_ context.Context, _ string, _ string, target domain.CloudCapacityTarget) error {
	f.prewarmed = append(f.prewarmed, target.ID)
	return nil
}

func (f *parallelPrewarmFake) Allocate(_ context.Context, _ string, _ string, target domain.CloudCapacityTarget) (domain.CloudProvisionedInstance, error) {
	if len(f.prewarmed) != f.want {
		return domain.CloudProvisionedInstance{}, fmt.Errorf("allocation began after only %d of %d targets were queued", len(f.prewarmed), f.want)
	}
	return domain.CloudProvisionedInstance{ID: "instance-" + target.ID, CapacityTargetID: target.ID, EnvironmentID: target.EnvironmentID, Status: "ready"}, nil
}

func (*parallelPrewarmFake) Release(context.Context, string, map[string]domain.RuntimeAllocation, bool) error {
	return nil
}

func (f *activityControllerFake) Inspect(_ context.Context, id string, _ domain.ExecutionMode) (*domain.ActivityHandle, error) {
	if f.inspections == nil && f.firstInspectStarts > 0 && len(f.started) != f.firstInspectStarts {
		return nil, fmt.Errorf("first inspection saw %d starts, want %d", len(f.started), f.firstInspectStarts)
	}
	if len(f.started) < f.minimumStartsBeforeInspect {
		return nil, fmt.Errorf("inspected after starting only %d activities", len(f.started))
	}
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
	service, err := New(store, activities, simulator, Config{PollInterval: time.Microsecond})
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

func TestSupervisorStartsTwelveActivitiesOnPlannedCapacity(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	request.Workflow.Activities = nil
	request.Workflow.Dependencies = nil
	request.Plan.Assignments = nil
	for index := range 12 {
		id := fmt.Sprintf("activity-%02d", index)
		request.Workflow.Activities = append(request.Workflow.Activities, domain.Activity{
			ID: id, Name: id, Kind: domain.ActivityKindTask,
			Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal},
			Command:      domain.ActivityCommand{Entrypoint: "true"},
			Resources:    domain.ActivityResources{CPU: 1},
		})
		request.Plan.Assignments = append(request.Plan.Assignments, domain.PlanAssignment{
			ID: "assignment-" + id, ActivityID: id, ResourceID: "r", CoreID: id,
		})
	}
	request.Resources[0].CPUCapacity = 12
	controller := &activityControllerFake{minimumStartsBeforeInspect: 12}
	service, err := New(&executionStoreFake{}, controller, &planExecutorFake{}, Config{
		PollInterval: time.Microsecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(controller.started) != 12 {
		t.Fatalf("started %d activities, want 12", len(controller.started))
	}
}

func TestSupervisorStartsIndependentActivitiesDespitePlanOrder(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	request.Workflow.Dependencies = nil
	request.Plan.Assignments[1].OrderOnResource = 1
	controller := &activityControllerFake{firstInspectStarts: 2}
	store := &executionStoreFake{}
	service, err := New(store, controller, &planExecutorFake{}, Config{PollInterval: time.Microsecond})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(controller.started) != 2 || controller.started[0] != "a" || controller.started[1] != "b" {
		t.Fatalf("start order=%v", controller.started)
	}
	for _, task := range store.tasks {
		if task.ActivityID == "b" && task.Metadata["queueReason"] == "waiting for previous activity in planned lane" {
			t.Fatal("plan order must not become a dependency")
		}
	}
}

func TestSupervisorStartsReadyActivityAheadOfBlockedPlanOrder(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	producer := request.Workflow.Activities[0]
	producer.ID, producer.Name = "c", "c"
	request.Workflow.Activities = append(request.Workflow.Activities, producer)
	request.Workflow.Dependencies = []domain.ActivityDependency{{
		ActivityID: "a", DependsOnActivityID: "c",
	}}
	request.Plan.Assignments[1].OrderOnResource = 1
	request.Plan.Assignments = append(request.Plan.Assignments, domain.PlanAssignment{
		ID: "pc", ActivityID: "c", ResourceID: "other",
	})
	request.Resources = append(request.Resources, domain.Resource{ID: "other"})
	request.RuntimeBindings = append(request.RuntimeBindings, domain.ResourceRuntimeBinding{
		ResourceID: "other", RuntimeID: "local", Enabled: true,
	})
	controller := &activityControllerFake{firstInspectStarts: 2}
	service, err := New(&executionStoreFake{}, controller, &planExecutorFake{}, Config{
		PollInterval: time.Microsecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(controller.started) != 3 || controller.started[0] == "a" || controller.started[1] == "a" || controller.started[2] != "a" {
		t.Fatalf("blocked activity ran before its dependency; starts=%v", controller.started)
	}
}

func TestSupervisorFollowsParallelPlanLanesDespiteOvercommit(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	request.Workflow.Dependencies = nil
	request.Workflow.Activities[0].Resources.CPU = 1
	request.Workflow.Activities[1].Resources.CPU = 1
	request.Resources[0].CPUCapacity = 1
	request.Plan.Assignments[0].SlotID = "slot-a"
	request.Plan.Assignments[1].SlotID = "slot-b"
	controller := &activityControllerFake{firstInspectStarts: 2}
	service, err := New(&executionStoreFake{}, controller, &planExecutorFake{}, Config{PollInterval: time.Microsecond})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
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
		PollInterval: time.Microsecond, Cloud: cloud,
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
	if len(store.tasks) == 0 || store.tasks[len(store.tasks)-1].CloudInstanceID != "instance-a" {
		t.Fatalf("task did not persist allocation: %#v", store.tasks)
	}
}

func TestSupervisorQueuesAllReadyCloudTargetsBeforeWaitingForFirst(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	request.Workflow.Dependencies = nil
	request.Workflow.Activities = append(request.Workflow.Activities,
		domain.Activity{ID: "c", Name: "c", Kind: domain.ActivityKindTask, Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal}, Command: domain.ActivityCommand{Entrypoint: "true"}},
		domain.Activity{ID: "d", Name: "d", Kind: domain.ActivityKindTask, Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal}, Command: domain.ActivityCommand{Entrypoint: "true"}},
	)
	request.Plan.Assignments = nil
	request.Resources = nil
	request.RuntimeBindings = nil
	request.Runtimes = []domain.EnvironmentRuntime{{ID: "cloud-runtime", Driver: domain.RuntimeDriverCloud, Mode: domain.RuntimeModeExecution}}
	targets := map[string]domain.CloudCapacityTarget{}
	for index, activity := range request.Workflow.Activities {
		resourceID := fmt.Sprintf("resource-%d", index)
		targetID := fmt.Sprintf("target-%d", index)
		request.Plan.Assignments = append(request.Plan.Assignments, domain.PlanAssignment{ID: "assignment-" + activity.ID, ActivityID: activity.ID, ResourceID: resourceID, Metadata: map[string]any{"runtimeId": "cloud-runtime"}})
		request.Resources = append(request.Resources, domain.Resource{ID: resourceID, Type: domain.ResourceCloudVM, Metadata: map[string]any{"capacityTargetId": targetID}})
		request.RuntimeBindings = append(request.RuntimeBindings, domain.ResourceRuntimeBinding{ResourceID: resourceID, RuntimeID: "cloud-runtime", Enabled: true})
		targets[targetID] = domain.CloudCapacityTarget{ID: targetID, EnvironmentID: "environment"}
	}
	allocator := &parallelPrewarmFake{want: 4}
	activities := &activityControllerFake{}
	service, err := New(&executionStoreFake{}, activities, &planExecutorFake{}, Config{
		PollInterval: time.Microsecond,
		Cloud:        &cloudProvisionerFake{}, CloudStore: cloudAllocationStoreFake{targets: targets}, CloudAllocator: allocator,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(allocator.prewarmed) != 4 || len(activities.started) != 4 {
		t.Fatalf("prewarmed=%v started=%v", allocator.prewarmed, activities.started)
	}
}

func TestSupervisorStartsReadyMachineWhileAnotherIsConfiguring(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	request.Workflow.Dependencies = nil
	request.Plan.Assignments = nil
	request.Resources = nil
	request.RuntimeBindings = nil
	request.Runtimes = []domain.EnvironmentRuntime{{ID: "cloud-runtime", Driver: domain.RuntimeDriverCloud, Mode: domain.RuntimeModeExecution}}
	targets := map[string]domain.CloudCapacityTarget{}
	for index, activity := range request.Workflow.Activities {
		resourceID := fmt.Sprintf("resource-%d", index)
		targetID := fmt.Sprintf("target-%d", index)
		request.Plan.Assignments = append(request.Plan.Assignments, domain.PlanAssignment{ID: "assignment-" + activity.ID, ActivityID: activity.ID, ResourceID: resourceID, Metadata: map[string]any{"runtimeId": "cloud-runtime"}})
		request.Resources = append(request.Resources, domain.Resource{ID: resourceID, Type: domain.ResourceCloudVM, Metadata: map[string]any{"capacityTargetId": targetID}})
		request.RuntimeBindings = append(request.RuntimeBindings, domain.ResourceRuntimeBinding{ResourceID: resourceID, RuntimeID: "cloud-runtime", Enabled: true})
		targets[targetID] = domain.CloudCapacityTarget{ID: targetID, EnvironmentID: "environment"}
	}
	slowReady := make(chan struct{})
	starts := make(chan string, 2)
	controller := &activityControllerFake{startEvents: starts}
	service, err := New(&executionStoreFake{}, controller, &planExecutorFake{}, Config{
		PollInterval: time.Millisecond,
		Cloud:        &cloudProvisionerFake{}, CloudStore: cloudAllocationStoreFake{targets: targets},
		CloudAllocator: &staggeredCloudAllocatorFake{slowReady: slowReady},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, executeErr := service.Execute(ctx, request)
		done <- executeErr
	}()
	select {
	case started := <-starts:
		if started != "b" {
			t.Fatalf("activity %q started before its machine was ready", started)
		}
	case <-ctx.Done():
		t.Fatal("ready machine did not start while another allocation was pending")
	}
	close(slowReady)
	if err := <-done; err != nil {
		t.Fatal(err)
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

func TestRunningTaskSeparatesTransferElapsedWorkAndWaits(t *testing.T) {
	task := newRunningTask(
		"run",
		"activity",
		domain.PlanAssignment{ID: "assignment", ResourceID: "resource"},
		domain.Resource{ID: "resource"},
		domain.RuntimeAllocation{},
		domain.ActivityHandle{StartedAt: 30},
		10,
		&domain.PreparationGate{TransferRuns: []domain.DataTransferRun{
			{StartedAt: 12, FinishedAt: 20, TransferredBytes: 100},
			{StartedAt: 14, FinishedAt: 24, TransferredBytes: 200},
		}},
	)
	if task.DataReadyAt != 24 || task.TransferSeconds != 18 || task.TransferBytes != 300 {
		t.Fatalf("transfer timing=%+v", task)
	}
	if task.Metadata["transferElapsedSeconds"] != 12.0 || task.Metadata["transferWorkSeconds"] != 18.0 ||
		task.Metadata["readyWaitSeconds"] != 2.0 || task.Metadata["launchWaitSeconds"] != 6.0 {
		t.Fatalf("timing metadata=%+v", task.Metadata)
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
	failedTask := store.tasks[len(store.tasks)-1]
	if failedTask.Status != domain.TaskFailed {
		t.Fatalf("tasks=%+v, want a failed task", store.tasks)
	}
	if failedTask.FailureReason != "start activity: activity image is required for Kubernetes" {
		t.Fatalf("failure=%q", failedTask.FailureReason)
	}
	handle := store.handles["run:a"]
	if handle.Status != domain.HandleFailed {
		t.Fatalf("handle status=%q, want failed", handle.Status)
	}
}
