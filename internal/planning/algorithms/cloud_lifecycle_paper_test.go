package algorithms

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestLifecycleKeepsMOHEFTForecastAndAddsCommonBillingActions(t *testing.T) {
	plan := domain.SchedulePlan{ID: "paper",
		Metadata: map[string]any{"costModel": "moheft-task-additive"},
		Predicted: domain.PredictedMetrics{Cost: 123,
			MakespanSeconds: 20,
			Feasible:        true},
		Assignments: []domain.PlanAssignment{{ID: "a",
			ResourceID:              "cloud",
			PredictedStartAt:        12,
			PredictedFinishAt:       20,
			PredictedRuntimeSeconds: 8}}}
	original := plan.Predicted
	EnrichCloudLifecycle(&plan, []domain.Resource{{ID: "cloud", Type: domain.ResourceCloudVM, PricePerSecond: 1}})
	if plan.Predicted != original {
		t.Fatalf("paper forecast overwritten: %+v", plan.Predicted)
	}
	if len(plan.LifecycleActions) == 0 {
		t.Fatal("execution billing lifecycle missing")
	}
}
