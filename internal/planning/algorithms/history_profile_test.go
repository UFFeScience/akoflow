package algorithms

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestDurationUsesActivityHistoryOnlyForItsResource(t *testing.T) {
	profiles := []domain.ActivityResourceProfile{
		{ActivityTypeID: "same-type", ResourceID: "r", RuntimeSeconds: 20},
		{ActivityID: "observed", ResourceID: "r", RuntimeSeconds: 8},
	}
	activity := domain.Activity{ID: "observed", ActivityTypeID: "same-type", Simulation: &domain.ActivitySimulation{DurationSeconds: 10}}
	if got := duration(activity, domain.Resource{ID: "r"}, profiles); got != 8 {
		t.Fatalf("matched history = %v", got)
	}
	if got := duration(activity, domain.Resource{ID: "other"}, profiles); got != 10 {
		t.Fatalf("different resource = %v", got)
	}
	activity.ID = "same-family"
	if got := duration(activity, domain.Resource{ID: "r"}, profiles); got != 20 {
		t.Fatalf("different activity with same type = %v", got)
	}
}
