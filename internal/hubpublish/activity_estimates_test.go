package hubpublish

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestActivityEstimatesRequestsPinnedVersionAndScopesCredentials(t *testing.T) {
	var paths []string
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/versions/1/activity-estimates") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if !strings.Contains(r.URL.Path, "/desktop/") {
			if r.Header.Get("Authorization") != "" {
				t.Error("public request contained credentials")
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("private request lacked credentials")
		}
		var input struct {
			Activities map[string]ActivityHistoryProfile
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Activities["task"].Architecture != "amd64" {
			t.Errorf("invalid request: %#v, %v", input, err)
		}
		_, _ = w.Write([]byte(`{"task":{"medianSeconds":5,"minSeconds":4,"maxSeconds":6,"sampleCount":3},"other":{"medianSeconds":5,"minSeconds":4,"maxSeconds":6,"sampleCount":3}}`))
	})
	manager.state.AccessToken = "secret"
	estimates, err := manager.ActivityEstimates(context.Background(), definitionURI, map[string]ActivityHistoryProfile{"task": {Architecture: "amd64", Runtime: "Slurm"}})
	if err != nil || len(paths) != 2 || len(estimates) != 1 || estimates["task"].MedianSeconds != 5 {
		t.Fatalf("unexpected estimates: %v, %v, %v", estimates, paths, err)
	}
}
