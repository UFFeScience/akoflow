package algorithms

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFixedEvaluationUpdatesForecastWithoutChangingPlacement(t *testing.T) {
	r := planningRequestFixture()
	sink := &testSink{}
	require.NoError(t, (HEFT{}).Schedule(context.Background(), r, nil, nil, sink))
	before := sink.plans[0]
	for i := range r.Resources {
		r.Resources[i].ComputeSpeedup *= 2
	}
	after, err := EvaluateFixedPlan(r, before)
	require.NoError(t, err)
	require.Len(t, after.Assignments, len(before.Assignments))
	for i, a := range after.Assignments {
		require.Equal(t, before.Assignments[i].ActivityID, a.ActivityID)
		require.Equal(t, before.Assignments[i].ResourceID, a.ResourceID)
		require.Equal(t, before.Assignments[i].CoreID, a.CoreID)
		require.Equal(t, before.Assignments[i].OrderOnResource, a.OrderOnResource)
	}
	require.NotEqual(t, before.Predicted.MakespanSeconds, after.Predicted.MakespanSeconds)
	require.Equal(t, "fixed-placement-production-stage-evaluator", after.Metadata["evaluationMode"])
	before.Assignments = before.Assignments[:1]
	_, err = EvaluateFixedPlan(r, before)
	require.Error(t, err)
}
