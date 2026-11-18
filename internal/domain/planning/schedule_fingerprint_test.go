package planning

import "testing"

func TestScheduleFingerprintSeparatesAllocationFromAlgorithmProvenance(t *testing.T) {
	first := SchedulePlan{ID: "a", Algorithm: "heft", Assignments: []PlanAssignment{
		{ID: "first", ActivityID: "work", ResourceID: "node", CoreID: "core-1", PredictedFinishAt: 1},
	}}
	second := first
	second.ID, second.Algorithm = "b", "prism-time"
	second.Assignments = append([]PlanAssignment(nil), first.Assignments...)
	second.Assignments[0].ID = "other"
	second.Assignments[0].Metadata = map[string]any{"scheduleBasis": "algorithm"}
	metrics := PredictedMetrics{MakespanSeconds: 1, Feasible: true}
	fingerprint, err := ScheduleFingerprint(first, metrics)
	if err != nil {
		t.Fatal(err)
	}
	equivalent, err := ScheduleFingerprint(second, metrics)
	if err != nil || fingerprint != equivalent {
		t.Fatalf("equivalent schedules must match: %v", err)
	}
	second.Assignments[0].CoreID = "core-2"
	different, err := ScheduleFingerprint(second, metrics)
	if err != nil || different == fingerprint {
		t.Fatalf("different allocations must not match: %v", err)
	}
}
