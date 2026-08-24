package simgrid

import (
	"encoding/json"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRunnerInputPrefersActivityRuntimeAndSeparatesOverhead(t *testing.T) {
	request := ports.ExecutionRequest{
		Run: domain.ExecutionRun{ID: "run"},
		Plan: domain.SchedulePlan{ID: "plan", Assignments: []domain.PlanAssignment{{
			ID: "assignment", ActivityID: "activity", ResourceID: "resource",
			Metadata: map[string]any{"bootOverheadSeconds": 12.0, "containerOverheadSeconds": 3.0},
		}}},
		Workflow: domain.WorkflowVersion{Activities: []domain.Activity{{
			ID: "activity", ActivityTypeID: "shared-type",
			Metadata: map[string]any{"baseRuntimeSeconds": 1_000.0},
		}}},
		Resources: []domain.Resource{{ID: "resource", ComputeSpeedup: 10}},
		ActivityProfiles: []domain.ActivityResourceProfile{{
			ActivityTypeID: "shared-type", ResourceID: "resource", RuntimeSeconds: 0.178,
		}},
	}

	payload, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	require.Len(t, input.Tasks, 1)
	require.InDelta(t, 1e12, input.Tasks[0].FLOPs, 1)
	require.InDelta(t, 15, input.Tasks[0].OverheadSeconds, 1e-9)
}
