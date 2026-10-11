package workflow

import "testing"

func TestNormalizedProfileScalesReferenceWorkInsteadOfReplayingActivity(t *testing.T) {
	profile := ActivityResourceProfile{RuntimeSeconds: 2, ModelVersion: "reference-work-v1"}
	for _, work := range []float64{9, 100} {
		if got := profile.RuntimeFor(Activity{Simulation: &ActivitySimulation{DurationSeconds: work}}); got != 2*work {
			t.Fatalf("work %g: runtime %g", work, got)
		}
	}
	if got := profile.RuntimeFor(Activity{}); got != 0 {
		t.Fatalf("unknown work must not reuse a measured activity duration: %g", got)
	}
	profile.ModelVersion = "legacy"
	if got := profile.RuntimeFor(Activity{}); got != 2 {
		t.Fatalf("legacy absolute profile changed: %g", got)
	}
}
