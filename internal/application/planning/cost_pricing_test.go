package planning

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestRecordPlanPricingDistinguishesExplicitZeroFromMissing(t *testing.T) {
	for _, known := range []bool{true, false} {
		plan := domain.SchedulePlan{Assignments: []domain.PlanAssignment{{ResourceID: "resource"}}}
		recordPlanPricing(&plan, domain.PlanningRequest{Resources: []domain.Resource{{
			ID: "resource", Metadata: map[string]any{
				"pricePerSecondKnown": known,
				"priceSource":         "fixture",
				"priceObservedAt":     "2026-11-15",
			},
		}}})
		pricing := plan.Metadata["pricing"].(planPricing)
		rate := pricing.Compute["resource"].Rate
		if pricing.Complete != known || (rate != nil) != known {
			t.Fatalf("known %v, got %+v", known, pricing)
		}
		if known && (*rate != 0 || pricing.Compute["resource"].Source != "fixture" ||
			pricing.Compute["resource"].ObservedAt != "2026-11-15") {
			t.Fatalf("explicit zero or provenance was lost: %+v", pricing)
		}
	}
}
