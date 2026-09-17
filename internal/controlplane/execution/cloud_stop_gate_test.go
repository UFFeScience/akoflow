package execution

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type cloudStopperFake struct {
	stopped  map[string]domain.RuntimeAllocation
	released map[string]domain.RuntimeAllocation
}

func (*cloudStopperFake) Allocate(context.Context, string, string, domain.CloudCapacityTarget) (domain.CloudProvisionedInstance, error) {
	return domain.CloudProvisionedInstance{}, nil
}

func (f *cloudStopperFake) Stop(_ context.Context, _ string, allocations map[string]domain.RuntimeAllocation) error {
	f.stopped = allocations
	return nil
}

func (f *cloudStopperFake) Release(_ context.Context, _ string, allocations map[string]domain.RuntimeAllocation, _ bool) error {
	f.released = allocations
	return nil
}

func TestReadyCloudStopsWaitsForDownstreamWorkspaceTransfer(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	request.Plan.Assignments[0].ResourceID = "cloud"
	request.Plan.Assignments[1].ResourceID = "local"
	request.Plan.LifecycleActions = []domain.PlannedLifecycleAction{{
		CapacityTargetID: "cloud", Action: "stop",
	}}
	request.Resources = []domain.Resource{{ID: "cloud"}, {ID: "local"}}
	request.RuntimeAllocations = map[string]domain.RuntimeAllocation{
		"a": {ResourceID: "cloud", CloudInstanceID: "vm"},
	}
	completed := map[string]domain.TaskExecution{"a": {ActivityID: "a", Status: domain.TaskCompleted}}
	tasks := map[string]domain.TaskExecution{"b": {ActivityID: "b", Status: domain.TaskQueued}}
	if got := readyCloudStops(request, completed, tasks, nil, true); len(got) != 0 {
		t.Fatalf("stopped before transfer: %v", got)
	}
	tasks["b"] = domain.TaskExecution{ActivityID: "b", Status: domain.TaskRunning}
	if got := readyCloudStops(request, completed, tasks, nil, false); len(got) != 0 {
		t.Fatalf("stopped without a transfer coordinator: %v", got)
	}
	if got := readyCloudStops(request, completed, tasks, nil, true); len(got) != 1 || len(got["vm"]) != 1 {
		t.Fatalf("eligible stops=%v, want vm", got)
	}
	if got := readyCloudStops(request, completed, tasks, map[string]bool{"vm": true}, true); len(got) != 0 {
		t.Fatalf("scheduled duplicate stop: %v", got)
	}
}

func TestReleaseCloudHonorsPlannedStopInsteadOfDestroyPolicy(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	request.Plan.Assignments = request.Plan.Assignments[:1]
	request.Plan.Assignments[0].ResourceID = "cloud"
	request.Resources = []domain.Resource{{ID: "cloud"}}
	request.Plan.LifecycleActions = []domain.PlannedLifecycleAction{{
		CapacityTargetID: "cloud", Action: "stop",
	}}
	request.RuntimeAllocations = map[string]domain.RuntimeAllocation{
		"a": {ResourceID: "cloud", CloudInstanceID: "vm"},
	}
	allocator := &cloudStopperFake{}
	supervisor := &Supervisor{config: Config{CloudAllocator: allocator}}
	if _, err := supervisor.releaseCloud(context.Background(), request, false); err != nil {
		t.Fatal(err)
	}
	if len(allocator.stopped) != 1 || len(allocator.released) != 0 {
		t.Fatalf("stopped=%v released=%v", allocator.stopped, allocator.released)
	}
}
