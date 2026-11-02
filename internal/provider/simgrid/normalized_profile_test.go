package simgrid

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestNormalizedMeasuredProfileOverridesOldReferenceAndPlanRuntime(t *testing.T) {
	activity := domain.Activity{ActivityTypeID: "family", Metadata: map[string]any{"baseRuntimeSeconds": 1}, Simulation: &domain.ActivitySimulation{DurationSeconds: 10}}
	resource := domain.Resource{ID: "micro", ComputeSpeedup: .25}
	profiles := map[string]domain.ActivityResourceProfile{"family\x00micro": {ActivityTypeID: "family", ResourceID: "micro", RuntimeSeconds: 2, ModelVersion: "reference-work-v1"}}
	if got := resolveRuntime(activity, domain.PlanAssignment{PredictedRuntimeSeconds: 999}, resource, profiles); got != 20 {
		t.Fatalf("expected normalized work 20 rather than old speed/reference/prediction, got %g", got)
	}
}
