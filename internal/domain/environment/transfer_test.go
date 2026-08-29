package environment

import (
	"testing"
	"time"
)

func TestCapabilityObservationFreshness(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	for _, observation := range []CapabilityObservation{
		{},
		{ObservedAt: now},
		{ObservedAt: now, FreshUntil: now.Add(time.Minute)},
		{ObservedAt: now, FreshUntil: now},
	} {
		if !observation.Fresh(now) {
			t.Fatalf("observation should be fresh: %+v", observation)
		}
	}
	if (CapabilityObservation{ObservedAt: now, FreshUntil: now.Add(time.Minute)}).Fresh(now.Add(2 * time.Minute)) {
		t.Fatal("expired observation reported as fresh")
	}
}
