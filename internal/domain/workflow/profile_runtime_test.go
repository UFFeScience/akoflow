package workflow

import "testing"

func TestIndependentComputeWorkDoesNotReuseOldPredictedSeconds(t *testing.T) {
	profile := ActivityResourceProfile{RuntimeSeconds: 4, ModelVersion: "compute-work-v1"}
	activity := Activity{Metadata: map[string]any{"computeWorkUnits": 100.0}, Simulation: &ActivitySimulation{DurationSeconds: 999}}
	if got := profile.RuntimeFor(activity); got != 400 {
		t.Fatalf("work-based runtime=%g", got)
	}
	activity.Simulation.DurationSeconds = 10
	if got := profile.RuntimeFor(activity); got != 400 {
		t.Fatalf("old machine-specific prediction changed work=%g", got)
	}
	delete(activity.Metadata, "computeWorkUnits")
	if got := profile.RuntimeFor(activity); got != 0 {
		t.Fatalf("missing work must fail closed, got %g", got)
	}
}

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
