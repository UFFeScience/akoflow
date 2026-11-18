package execution

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestProjectActivityTestRequestHasIndependentExecutionState(t *testing.T) {
	original := requestFixture(domain.ExecutionModeReal)
	original.Run.Status = domain.ExecutionRunCompleted
	original.PreflightID = "original-preflight"
	original.Recovery = &ports.ExecutionRecovery{Mode: "continue"}
	original.RuntimeAllocations = map[string]domain.RuntimeAllocation{"b": {CloudInstanceID: "source-instance"}}
	original.PreparationRequirementsByActivity = map[string]domain.PreparationRequirement{"b": {}}
	graph, err := PlanActivityTestGraph(original.Workflow, "b", map[string]bool{"a": true})
	if err != nil {
		t.Fatal(err)
	}
	request, err := ProjectActivityTestRequest(original, "activity-test", graph)
	if err != nil {
		t.Fatal(err)
	}
	if request.Run.ID != "activity-test" || request.Run.Status != domain.ExecutionRunCreated ||
		request.Run.SchedulePlanID != original.Plan.ID || request.Run.ActivityCount != 1 ||
		len(request.Workflow.Activities) != 1 || request.Workflow.Activities[0].ID != "b" ||
		len(request.Workflow.Dependencies) != 0 || len(request.Plan.Assignments) != 1 ||
		request.Plan.Assignments[0].ActivityID != "b" || request.Recovery != nil ||
		request.RuntimeAllocations != nil || request.PreparationRequirementsByActivity != nil ||
		request.PreflightID != "" {
		t.Fatalf("unexpected test request: %+v", request)
	}
	if original.Run.ID != "run" || original.Run.Status != domain.ExecutionRunCompleted ||
		len(original.Workflow.Dependencies) != 1 || len(original.Plan.Assignments) != 2 {
		t.Fatalf("original request changed: %+v", original)
	}
	if _, err := ProjectActivityTestRequest(original, original.Run.ID, graph); err == nil {
		t.Fatal("test must not use the original run ID")
	}
}
