package algorithms

import (
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/storagebudget"
	workflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
	"github.com/stretchr/testify/require"
)

func TestCompletePRISMCandidateRejectsExhaustedSharedQuota(t *testing.T) {
	a := planningActivity("a", 1)
	a.Metadata = map[string]any{"workspaceProjection": workflow.WorkspaceProjection{Contract: "whole-workspace-v1", Source: "measured", Outputs: []workflow.ProjectedFile{{Path: "out", ContentID: "out", SizeBytes: 100}}}}
	r := planningResource("node", 1, 1, 0)
	r.Metadata = map[string]any{"workspaceStorageBudget": storagebudget.Snapshot{StorageID: "user-home", AvailableBytes: 0, Source: "quota-api", ObservedAt: time.Now()}}
	request := domain.PlanningRequest{Workflow: domain.WorkflowVersion{Activities: []domain.Activity{a}}, Resources: []domain.Resource{r}, ExecutionScope: domain.ExecutionScope{EnvironmentVersionIDs: []string{"environment"}}}
	search, err := newCompactPRISMContext(request, nil)
	require.NoError(t, err)
	assignments := []domain.PlanAssignment{{ActivityID: "a", ResourceID: "node", CoreID: "node-core-1"}}
	_, err = evaluateCompleteCompactPRISMState(search, compactPRISMStateFromEvaluation(compactPRISMState{}, assignments, 0, 0))
	require.ErrorContains(t, err, "storage quota")
}

func TestQuotaInfeasibleCandidateDoesNotAbortOtherCandidates(t *testing.T) {
	a := planningActivity("a", 1)
	a.Metadata = map[string]any{"workspaceProjection": workflow.WorkspaceProjection{Contract: "whole-workspace-v1", Source: "measured", Outputs: []workflow.ProjectedFile{{Path: "out", ContentID: "out", SizeBytes: 100}}}}
	r := planningResource("full", 1, 1, 0)
	r.Metadata = map[string]any{"workspaceStorageBudget": storagebudget.Snapshot{StorageID: "full", AvailableBytes: 0, Source: "quota-api", ObservedAt: time.Now()}}
	r2 := planningResource("available", 1, 1, 0)
	r2.Metadata = map[string]any{"workspaceStorageBudget": storagebudget.Snapshot{StorageID: "available", AvailableBytes: 1000, Source: "quota-api", ObservedAt: time.Now()}}
	search, err := newCompactPRISMContext(domain.PlanningRequest{Workflow: domain.WorkflowVersion{Activities: []domain.Activity{a}}, Resources: []domain.Resource{r, r2}, ExecutionScope: domain.ExecutionScope{EnvironmentVersionIDs: []string{"environment"}}}, nil)
	require.NoError(t, err)
	states := []compactPRISMState{}
	for _, id := range []string{"full", "available"} {
		states = append(states, compactPRISMStateFromEvaluation(compactPRISMState{}, []domain.PlanAssignment{{ActivityID: "a", ResourceID: id, CoreID: id + "-core-1"}}, 0, 0))
	}
	got, err := reevaluateCompleteCompactPRISMStates(search, states)
	require.NoError(t, err)
	require.Len(t, got, 1)
	_, err = reevaluateCompleteCompactPRISMStates(search, states[:1])
	require.ErrorContains(t, err, "storage quota")
}
