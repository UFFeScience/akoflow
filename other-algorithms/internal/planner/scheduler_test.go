package planner

import "testing"

func TestMemoryAwarePrefersFeasibleMemoryAndFasterResource(t *testing.T) {
	input := testInput()
	envelope, err := BuildPlan(input, Options{
		Algorithm:             MemoryAware,
		PlanID:                "memory-plan",
		Alpha:                 0.8,
		DefaultRuntimeSeconds: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(envelope.Plan.Assignments) != 2 {
		t.Fatalf("got %d assignments", len(envelope.Plan.Assignments))
	}
	if envelope.Plan.Assignments[0].ResourceID != "large-fast" {
		t.Fatalf("first assignment resource = %q", envelope.Plan.Assignments[0].ResourceID)
	}
	if envelope.Plan.Assignments[1].PredictedStartAt < envelope.Plan.Assignments[0].PredictedFinishAt {
		t.Fatal("dependency was not respected")
	}
}

func TestFIFOUsesStableWorkflowOrder(t *testing.T) {
	input := testInput()
	input.Workflow.Dependencies = nil
	envelope, err := BuildPlan(input, Options{
		Algorithm:             FIFO,
		PlanID:                "fifo-plan",
		DefaultRuntimeSeconds: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Plan.Assignments[0].ActivityID != "first" || envelope.Plan.Assignments[1].ActivityID != "second" {
		t.Fatalf("FIFO order changed: %#v", envelope.Plan.Assignments)
	}
}

func testInput() Input {
	return Input{
		Workflow: WorkflowVersion{
			ID: "workflow-v1",
			Activities: []Activity{
				{ID: "first", ActivityTypeID: "task", Resources: ActivityResources{CPU: 1, MemoryBytes: 8}, Simulation: &Simulation{DurationSeconds: 10}},
				{ID: "second", ActivityTypeID: "task", Resources: ActivityResources{CPU: 1, MemoryBytes: 8}, Simulation: &Simulation{DurationSeconds: 10}},
			},
			Dependencies: []ActivityDependency{{ActivityID: "second", DependsOnActivityID: "first"}},
		},
		ExecutionScope: ExecutionScope{ID: "scope", EnvironmentVersionIDs: []string{"environment-v1"}},
		Resources: []Resource{
			{ID: "large-fast", EnvironmentVersionID: "environment-v1", CPUCores: 2, CPUCapacity: 2, MemoryBytes: 32, ComputeSpeedup: 2, Schedulable: true},
			{ID: "small-slow", EnvironmentVersionID: "environment-v1", CPUCores: 1, CPUCapacity: 1, MemoryBytes: 8, ComputeSpeedup: 1, Schedulable: true},
		},
		NetworkTopology: NetworkTopology{ID: "network", ExecutionScopeID: "scope"},
	}
}
