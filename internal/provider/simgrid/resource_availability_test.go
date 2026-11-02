package simgrid

import (
	"encoding/json"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestInitialAvailabilityIsAbsoluteAndSeparateFromOverhead(t *testing.T) {
	request := processRequestFixture()
	for i := range request.Plan.Assignments {
		request.Plan.Assignments[i].Metadata = map[string]any{"resourceAvailableAtSeconds": 12.0, "bootOverheadSeconds": 0.0, "containerOverheadSeconds": 2.0}
	}
	payload, err := buildRunnerInput(request, 1e9)
	require.NoError(t, err)
	var input runnerInput
	require.NoError(t, json.Unmarshal(payload, &input))
	for _, task := range input.Tasks {
		require.Equal(t, 12.0, task.ResourceAvailableAtSeconds)
		require.Equal(t, 2.0, task.OverheadSeconds)
	}
	for _, value := range []any{-1.0, math.Inf(1), math.NaN(), "12"} {
		_, err := assignmentResourceAvailableAt(domain.PlanAssignment{Metadata: map[string]any{"resourceAvailableAtSeconds": value}})
		require.Error(t, err)
	}
}
