package execution

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestRestrictedCapacityReservesOneMicroLane(t *testing.T) {
	assignments := map[string]domain.PlanAssignment{"a": {ResourceID: "micro"}, "b": {ResourceID: "micro"}}
	resources := map[string]domain.Resource{"micro": {CPUCores: 2, CPUCapacity: 1}}
	if got := restrictedCapacityReady([]string{"a", "b"}, assignments, resources, nil, nil); len(got) != 1 || got[0] != "a" {
		t.Fatalf("admission=%v", got)
	}
	for _, status := range []domain.TaskExecutionStatus{domain.TaskPreparing, domain.TaskRunning} {
		tasks := map[string]domain.TaskExecution{"a": {Status: status}}
		if got := restrictedCapacityReady([]string{"b"}, assignments, resources, tasks, nil); len(got) != 0 {
			t.Fatalf("busy admission=%v", got)
		}
	}
	tasks := map[string]domain.TaskExecution{"a": {Status: domain.TaskQueued}}
	if got := restrictedCapacityReady([]string{"b"}, assignments, resources, tasks, map[string]bool{"a": true}); len(got) != 0 {
		t.Fatalf("allocation admission=%v", got)
	}
	tasks["a"] = domain.TaskExecution{Status: domain.TaskCompleted}
	if got := restrictedCapacityReady([]string{"b"}, assignments, resources, tasks, nil); len(got) != 1 {
		t.Fatalf("released admission=%v", got)
	}
}

func TestRequestRejectsPlanExceedingExplicitMicroCapacity(t *testing.T) {
	request := requestFixture(domain.ExecutionModeReal)
	request.Resources[0].CPUCores = 2
	request.Resources[0].CPUCapacity = 1
	request.Plan.Assignments[0].CoreID = "r-core-1"
	request.Plan.Assignments[1].CoreID = "r-core-2"
	if err := validateRequest(request); err == nil {
		t.Fatal("oversubscribed micro plan accepted")
	}
	request.Plan.Assignments[1].CoreID = "r-core-1"
	if err := validateRequest(request); err != nil {
		t.Fatal(err)
	}
}

func TestRestrictedCapacityReservesOneBatchLane(t *testing.T) {
	assignments := map[string]domain.PlanAssignment{"a": {ResourceID: "hpc"}, "b": {ResourceID: "hpc"}}
	resources := map[string]domain.Resource{"hpc": {CPUCores: 64, CPUCapacity: 1, ExecutionTarget: domain.ExecutionTargetBatch}}
	if got := restrictedCapacityReady([]string{"a", "b"}, assignments, resources, nil, nil); len(got) != 1 {
		t.Fatalf("batch admission exceeded explicit capacity: %v", got)
	}
	for _, status := range []domain.TaskExecutionStatus{domain.TaskPreparing, domain.TaskRunning} {
		if got := restrictedCapacityReady([]string{"b"}, assignments, resources, map[string]domain.TaskExecution{"a": {Status: status}}, nil); len(got) != 0 {
			t.Fatalf("occupied batch lane admitted another activity: %v", got)
		}
	}
	if got := restrictedCapacityReady([]string{"b"}, assignments, resources, map[string]domain.TaskExecution{"a": {Status: domain.TaskCompleted}}, nil); len(got) != 1 {
		t.Fatalf("completed batch lane was not released: %v", got)
	}
}

func TestCloudCapacityEqualToPhysicalCoresStillLimitsAdmission(t *testing.T) {
	assignments := map[string]domain.PlanAssignment{"a": {ResourceID: "vm"}, "b": {ResourceID: "vm"}, "c": {ResourceID: "vm"}}
	resources := map[string]domain.Resource{"vm": {Type: domain.ResourceCloudVM, CPUCores: 2, CPUCapacity: 2}}
	if got := restrictedCapacityReady([]string{"a", "b", "c"}, assignments, resources, nil, nil); len(got) != 2 {
		t.Fatalf("cloud admission=%v, want two slots", got)
	}
	tasks := map[string]domain.TaskExecution{"a": {Status: domain.TaskPreparing}, "b": {Status: domain.TaskRunning}}
	if got := restrictedCapacityReady([]string{"c"}, assignments, resources, tasks, nil); len(got) != 0 {
		t.Fatalf("busy cloud admitted=%v", got)
	}
	tasks["b"] = domain.TaskExecution{Status: domain.TaskCompleted}
	if got := restrictedCapacityReady([]string{"c"}, assignments, resources, tasks, nil); len(got) != 1 {
		t.Fatalf("released cloud slot admission=%v", got)
	}
}
