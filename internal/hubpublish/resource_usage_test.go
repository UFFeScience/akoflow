package hubpublish

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCaptureResourceUsagePreservesAttemptsAndKeepsWindowEndpoints(t *testing.T) {
	source := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/akoflow-api/execution-runs/run/activities/task/metrics/" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		items := make([]map[string]any, 480)
		for index := range items {
			items[index] = map[string]any{"observedAt": 10 + index, "memoryBytes": 0, "internal": "not part of the export"}
		}
		if r.URL.Query().Get("attempt") == "2" {
			items = items[:1]
			delete(items[0], "memoryBytes")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	})
	manager := &Manager{options: Options{Source: source}}
	summaries := []any{
		map[string]any{"activityId": "task", "attempt": float64(1), "samples": float64(480)},
		map[string]any{"activityId": "task", "attempt": float64(2), "samples": float64(1)},
	}
	result, err := manager.captureResourceUsage(context.Background(), "/execution-runs/run/", summaries)
	if err != nil {
		t.Fatal(err)
	}
	groups := result["groups"].([]any)
	first := groups[0].(map[string]any)["samples"].([]any)
	second := groups[1].(map[string]any)["samples"].([]any)
	if len(first) != 240 || result["includedSamples"] != 241 || result["observedSamples"] != 481 {
		t.Fatalf("incorrect sample bounds: %v / %v", result["includedSamples"], result["observedSamples"])
	}
	if first[0].(map[string]any)["observedAt"] != float64(10) ||
		first[len(first)-1].(map[string]any)["observedAt"] != float64(489) ||
		first[0].(map[string]any)["memoryBytes"] != float64(0) ||
		second[0].(map[string]any)["memoryBytes"] != nil ||
		first[0].(map[string]any)["internal"] != nil {
		t.Fatal("sampling changed endpoints, zero or unavailable values, or exported a private field")
	}
	record := map[string]any{"activityMetricSeries": result}
	if err := selectExecutionContent(record, ContentSelection{}); err != nil || record["activityMetricSeries"] != nil {
		t.Fatal("unchecked selection included resource samples")
	}
	record["activityMetricSeries"] = result
	if err := selectExecutionContent(record, ContentSelection{ResourceUsage: true}); err != nil || record["activityMetricSeries"] == nil {
		t.Fatal("selected resource samples were omitted")
	}
}

func TestCaptureResourceUsageHandlesHistoricalRunsWithoutSamples(t *testing.T) {
	manager := &Manager{}
	result, err := manager.captureResourceUsage(context.Background(), "/execution-runs/legacy/", nil)
	if err != nil || result["includedSamples"] != 0 || len(result["groups"].([]any)) != 0 {
		t.Fatalf("historical execution received unexpected usage: %v (%v)", result, err)
	}
}
