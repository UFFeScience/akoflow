package hubpublish

import "testing"

func TestImageObservationsKeepAttemptIdentityWithoutProcessLogs(t *testing.T) {
	first := map[string]any{"imageObservation": map[string]any{
		"status": "unavailable", "requestedReference": "fixture:mutable",
	}}
	second := map[string]any{"imageObservation": map[string]any{
		"status": "observed", "runtimeImageId": "sha256:retry", "platform": "linux/amd64",
	}}
	detail := map[string]any{
		"run": map[string]any{"id": "run"},
		"activityAttempts": []any{
			map[string]any{"activityId": "activity", "attempt": float64(1)},
			map[string]any{"activityId": "activity", "attempt": float64(2)},
		},
		"handles": []any{
			map[string]any{"id": "run:activity", "metadata": first, "log": "excluded"},
			map[string]any{"id": "run:activity:attempt-2", "metadata": second},
		},
	}
	execution := exportExecution(detail, nil)
	if err := selectExecutionContent(execution, ContentSelection{}); err != nil {
		t.Fatal(err)
	}
	observations, _ := execution["imageObservations"].([]map[string]any)
	if len(observations) != 2 || observations[0]["status"] != "unavailable" ||
		observations[1]["attempt"] != float64(2) || observations[1]["runtimeImageId"] != "sha256:retry" ||
		execution["processLogs"] != nil {
		t.Fatalf("unexpected exported evidence: %+v", execution)
	}
}
