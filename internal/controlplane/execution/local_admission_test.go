package execution

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestLocalAllocationPreventsParallelismWithOneCpu(t *testing.T) {
	assignments := map[string]domain.PlanAssignment{"a": {ResourceID: "local"}, "b": {ResourceID: "local"}}
	resources := map[string]domain.Resource{"local": {
		ID: "local", Type: domain.ResourceLocalMachine, CPUCores: 1, CPUCapacity: 1, MemoryBytes: 1 << 30,
		Metadata: map[string]any{"localAllocation": true},
	}}
	got := restrictedCapacityReady([]string{"a", "b"}, assignments, resources, nil, nil)
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("one CPU admitted parallel tasks: %v", got)
	}
}

func TestLocalAdmissionReservesMemoryDuringPreparationAndReleasesIt(t *testing.T) {
	assignments := map[string]domain.PlanAssignment{"a": {ResourceID: "local"}, "b": {ResourceID: "local"}}
	resources := map[string]domain.Resource{"local": {
		ID: "local", Type: domain.ResourceLocalMachine, CPUCores: 4, CPUCapacity: 4, MemoryBytes: 1 << 30,
		Metadata: map[string]any{"localAllocation": true},
	}}
	activities := map[string]domain.Activity{
		"a": {Resources: domain.ActivityResources{CPU: 1, MemoryBytes: 700 << 20}},
		"b": {Resources: domain.ActivityResources{CPU: 1, MemoryBytes: 700 << 20}},
	}
	for _, status := range []domain.TaskExecutionStatus{domain.TaskPreparing, domain.TaskRunning, domain.TaskQueued} {
		tasks := map[string]domain.TaskExecution{"a": {Status: status}}
		got := localCapacityReady([]string{"b"}, assignments, resources, activities, tasks, map[string]bool{"a": true})
		if len(got) != 0 {
			t.Fatalf("memory overcommitted for %s: %v", status, got)
		}
	}
	got := localCapacityReady([]string{"a", "b"}, assignments, resources, activities, nil, nil)
	if len(got) != 1 {
		t.Fatalf("simultaneous reservations overcommitted memory: %v", got)
	}
	tasks := map[string]domain.TaskExecution{"a": {Status: domain.TaskCompleted}}
	if got := localCapacityReady([]string{"b"}, assignments, resources, activities, tasks, nil); len(got) != 1 {
		t.Fatalf("completed task retained memory: %v", got)
	}
	activities["a"] = domain.Activity{Resources: domain.ActivityResources{CPU: 3}}
	activities["b"] = domain.Activity{Resources: domain.ActivityResources{CPU: 2}}
	if got := localCapacityReady([]string{"a", "b"}, assignments, resources, activities, nil, nil); len(got) != 1 {
		t.Fatalf("CPU overcommitted: %v", got)
	}
}
