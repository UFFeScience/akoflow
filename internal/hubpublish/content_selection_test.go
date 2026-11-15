package hubpublish

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSelectionRemovesOptionalContentAndRehashesBeforeUpload(t *testing.T) {
	workflow := map[string]any{"id": "workflow", "version": map[string]any{"id": "v1"}}
	execution := map[string]any{
		"run":             map[string]any{"id": "run"},
		"processLogs":     []any{map[string]any{"log": "secret-log-marker"}},
		"dataObjects":     []any{"file-marker"},
		"files":           []any{"file-marker"},
		"dataLocations":   []any{"location-marker"},
		"events":          []any{"event-marker"},
		"timeline":        []any{"timeline-marker"},
		"metricSummaries": []any{"metric-marker"},
	}
	capture, err := encodeExperiment(workflow, []any{}, execution, "name: Workflow", nil)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := selectCapture(capture, ContentSelection{})
	if err != nil {
		t.Fatal(err)
	}
	bytes, _ := json.Marshal(selected.bundle)
	for _, marker := range []string{"secret-log-marker", "file-marker", "location-marker", "event-marker", "timeline-marker", "metric-marker"} {
		if strings.Contains(string(bytes), marker) {
			t.Fatalf("unselected data leaked: %s", marker)
		}
	}
	for _, file := range selected.bundle.Files {
		if digest([]byte(file.Content)) != file.SHA256 {
			t.Fatal("stale checksum")
		}
	}
	if selected.execution["run"] == nil || len(selected.bundle.Files) != 4 {
		t.Fatal("required experiment context removed")
	}
}

func TestSelectionRetainsOnlyAcceptedSections(t *testing.T) {
	workflow := map[string]any{"id": "workflow", "version": map[string]any{"id": "v1"}}
	execution := map[string]any{
		"processLogs":     []any{"accepted-log"},
		"files":           []any{"rejected-file"},
		"metricSummaries": []any{"accepted-metric"},
		"events":          []any{"rejected-event"},
	}
	capture, err := encodeExperiment(workflow, []any{}, execution, "name: Workflow", nil)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := selectCapture(capture, ContentSelection{Logs: true, Metrics: true})
	if err != nil {
		t.Fatal(err)
	}
	bytes, _ := json.Marshal(selected.bundle)
	if !strings.Contains(string(bytes), "accepted-log") || !strings.Contains(string(bytes), "accepted-metric") || strings.Contains(string(bytes), "rejected-") {
		t.Fatal("selection not respected")
	}
}
