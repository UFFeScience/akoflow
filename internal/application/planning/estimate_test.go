package planning

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestPRISMEstimateUsesWorkflowBeamReadyWidthAndResources(t *testing.T) {
	workflow := domain.WorkflowVersion{
		Activities: []domain.Activity{
			{ID: "root"}, {ID: "left"}, {ID: "right"}, {ID: "join"},
		},
		Dependencies: []domain.ActivityDependency{
			{ActivityID: "left", DependsOnActivityID: "root"},
			{ActivityID: "right", DependsOnActivityID: "root"},
			{ActivityID: "join", DependsOnActivityID: "left"},
			{ActivityID: "join", DependsOnActivityID: "right"},
		},
	}
	scope := domain.ExecutionScope{EnvironmentVersionIDs: []string{"environment"}}
	resources := []domain.Resource{
		{ID: "m1", EnvironmentVersionID: "environment", Schedulable: true, CPUCapacity: 1},
		{ID: "m2", EnvironmentVersionID: "environment", Schedulable: true, CPUCapacity: 1},
	}
	estimate := estimateAlgorithmRun(
		"prism-time",
		map[string]any{"beamWidth": 10, "readyBranchLimit": 3},
		workflow,
		scope,
		resources,
	)
	if estimate.ActivityCount != 4 || estimate.DependencyCount != 4 {
		t.Fatalf("unexpected workflow size: %+v", estimate)
	}
	if estimate.ReadyBranchLimit != 2 {
		t.Fatalf("expected ready width to be capped by DAG parallelism, got %d", estimate.ReadyBranchLimit)
	}
	if estimate.ExpandedStates != 160 {
		t.Fatalf("expected 4*10*2*2 expansions, got %d", estimate.ExpandedStates)
	}
	if estimate.DurationSeconds <= 0 {
		t.Fatalf("expected a positive duration estimate, got %.3f", estimate.DurationSeconds)
	}
}

func TestHEFTEstimateDoesNotUseBeam(t *testing.T) {
	workflow := domain.WorkflowVersion{Activities: []domain.Activity{{ID: "a"}, {ID: "b"}}}
	scope := domain.ExecutionScope{EnvironmentVersionIDs: []string{"environment"}}
	resources := []domain.Resource{
		{ID: "m1", EnvironmentVersionID: "environment", Schedulable: true, CPUCapacity: 2, CPUCores: 2},
	}
	estimate := estimateAlgorithmRun("heft", map[string]any{"beamWidth": 999}, workflow, scope, resources)
	if estimate.BeamWidth != 0 || estimate.ExpandedStates != 4 {
		t.Fatalf("unexpected HEFT estimate: %+v", estimate)
	}
}
