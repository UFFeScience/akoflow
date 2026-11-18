package hubpublish

import (
	"strings"
	"testing"
)

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

func TestExportImageObservationIncludesOnlyVerifiedSIFProvenance(t *testing.T) {
	sif := "sha256:" + strings.Repeat("a", 64)
	source := "sha256:" + strings.Repeat("b", 64)
	observation := map[string]any{"status": "observed", "sifDigest": sif, "requestedReference": "/private/tool.sif"}
	link := map[string]any{"digest": sif, "sourceDigest": source, "sourceVariantId": "private-variant"}
	execution := map[string]any{
		"run":        map[string]any{"id": "run"},
		"activities": []any{map[string]any{"activityId": "activity"}},
	}
	handles := []any{map[string]any{"id": "run:activity", "metadata": map[string]any{
		"imageObservation": observation, "executionImageArtifact": link,
	}}}
	result := exportImageObservations(execution, handles)[0]
	if result["sifDigest"] != sif || result["sourceOCIDigest"] != source ||
		result["requestedReference"] != nil || result["sourceVariantId"] != nil {
		t.Fatalf("unexpected projection: %+v", result)
	}
	link["digest"] = source
	if result := exportImageObservations(execution, handles)[0]; result["sourceOCIDigest"] != nil {
		t.Fatalf("source from a different artifact was exported: %v", result)
	}
	observation["status"] = "unavailable"
	if result := exportImageObservations(execution, handles)[0]; result["sifDigest"] != nil {
		t.Fatalf("unobserved SIF was exported: %v", result)
	}
}
