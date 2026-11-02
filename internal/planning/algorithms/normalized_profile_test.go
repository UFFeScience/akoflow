package algorithms

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestPlannerUsesSameNormalizedMeasuredWorkAsSimulation(t *testing.T) {
	activity := domain.Activity{ActivityTypeID: "family", Simulation: &domain.ActivitySimulation{DurationSeconds: 10}}
	resource := domain.Resource{ID: "micro", ComputeSpeedup: .25}
	profiles := []domain.ActivityResourceProfile{{ActivityTypeID: "family", ResourceID: "micro", RuntimeSeconds: 2, ModelVersion: "reference-work-v1"}}
	if got := duration(activity, resource, profiles); got != 20 {
		t.Fatalf("expected normalized work 20, got %g", got)
	}
}
