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
