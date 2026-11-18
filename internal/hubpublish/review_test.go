package hubpublish

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHubConfirmationRequiresReviewedVisibilityAndExactSnapshot(t *testing.T) {
	visibility := "public"
	confirmations := 0
	reviewed := "exact immutable reviewed bytes"
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/desktop/me":
			_ = json.NewEncoder(w).Encode(Identity{Username: "researcher"})
		case "/api/v1/desktop/publications/publication":
			_ = json.NewEncoder(w).Encode(publicationInfo{Publication: Publication{
				ID: "publication", Visibility: visibility, Owner: "researcher", Slug: "experiment",
			}})
		case "/api/v1/desktop/publications/publication/import/confirm":
			var input importInput
			if json.NewDecoder(r.Body).Decode(&input) != nil || input.BundleFile != reviewed || input.BaseVersion != 3 {
				t.Error("reviewed bytes changed")
			}
			confirmations++
			_ = json.NewEncoder(w).Encode(map[string]any{"experimentId": "capture", "version": map[string]int{"number": 4}, "resultCount": 1})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	})
	manager.state.AccessToken = strings.Repeat("c", 64)
	manager.previews["review"] = preparedPublication{
		Input:       importInput{BundleFile: reviewed, BaseVersion: 3},
		Publication: Publication{ID: "publication", Visibility: "public"},
		TokenHash:   digest([]byte(manager.state.AccessToken)), ExpiresAt: time.Now().Add(time.Minute),
	}
	ctx := context.Background()
	if _, err := manager.Confirm(ctx, "review", false); err == nil {
		t.Fatal("public capture without acknowledgement")
	}
	visibility = "private"
	if _, err := manager.Confirm(ctx, "review", true); err == nil {
		t.Fatal("changed visibility accepted")
	}
	visibility = "public"
	receipt, err := manager.Confirm(ctx, "review", true)
	if err != nil || receipt.Version != 4 || !strings.HasSuffix(receipt.URL, "/u/researcher/experiment") {
		t.Fatalf("confirmation: %+v %v", receipt, err)
	}
	manager.state.AccessToken = strings.Repeat("d", 64)
	if _, err := manager.Confirm(ctx, "review", true); err == nil {
		t.Fatal("review reused by another identity")
	}
	if confirmations != 1 {
		t.Fatalf("unexpected uploads: %d", confirmations)
	}
}
func TestHubRedactionPreservesScientificNumbersAndCredentialReferences(t *testing.T) {
	raw := map[string]any{
		"password": "private-observation-secret", "cores": float64(8),
		"log":        "password=hidden Bearer abcdefghijklmnop private-observation-secret",
		"credential": "credential://cloud/key", "uri": "https://storage.example/a?X-Goog-Signature=secret&part=1",
	}
	cleaned, paths, err := redactSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(cleaned)
	for _, secret := range []string{"private-observation-secret", "abcdefghijklmnop", "=hidden", "Signature=secret"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("unredacted secret")
		}
	}
	snapshot := cleaned.(map[string]any)
	if snapshot["cores"] != float64(8) || snapshot["credential"] != "credential://cloud/key" || len(paths) < 3 {
		t.Fatalf("scientific data changed: %+v", snapshot)
	}
}

func TestHubPreviewFiltersOversizedLogsBeforeEnforcingLimit(t *testing.T) {
	var reviewed string
	var confirmed bool
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/desktop/me":
			_ = json.NewEncoder(w).Encode(Identity{Username: "researcher"})
		case "/api/v1/desktop/publications/publication":
			_ = json.NewEncoder(w).Encode(publicationInfo{Publication: Publication{
				ID: "publication", Visibility: "private", Owner: "researcher", Slug: "experiment",
			}})
		case "/api/v1/desktop/publications/publication/import/preview",
			"/api/v1/desktop/publications/publication/import/confirm":
			var input importInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/preview") {
				reviewed = input.BundleFile
			} else {
				confirmed = input.BundleFile == reviewed
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"version": map[string]int{"number": 1}})
		default:
			t.Errorf("unexpected Hub request: %s", r.URL.Path)
		}
	})
	manager.state.AccessToken = strings.Repeat("c", 64)
	manager.options.Source = oversizedLogSource(t)
	ctx := context.Background()
	_, err := manager.Preview(ctx, "run", "publication", "", &ContentSelection{Logs: true})
	if failure, ok := err.(*apiError); !ok || failure.Code != "package_too_large" || reviewed != "" {
		t.Fatalf("expected selected capture to be rejected locally, got %v", err)
	}
	preview, err := manager.Preview(ctx, "run", "publication", "", &ContentSelection{})
	if err != nil {
		t.Fatal(err)
	}
	var bundle experimentBundle
	if err := json.Unmarshal([]byte(reviewed), &bundle); err != nil {
		t.Fatal(err)
	}
	if len(preview.Logs) != 0 || len(bundle.Files) != 4 || strings.Contains(reviewed, "oversized-log-marker") {
		t.Fatal("review did not preserve the required files and omit the deselected logs")
	}
	for _, file := range bundle.Files {
		if digest([]byte(file.Content)) != file.SHA256 {
			t.Fatalf("invalid checksum for %s", file.Path)
		}
	}
	if _, err := manager.Confirm(ctx, preview.ID, false); err != nil || !confirmed {
		t.Fatalf("confirmation did not submit the exact reviewed package: %v", err)
	}
}

func oversizedLogSource(t *testing.T) http.Handler {
	run := map[string]any{"id": "run", "schedulePlanId": "plan", "workflowVersionId": "version", "status": "completed"}
	workflow := map[string]any{"id": "workflow", "name": "Workflow", "version": map[string]any{"id": "version"}}
	log := "oversized-log-marker " + strings.Repeat("x", packageLimit)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var value any
		switch r.URL.Path {
		case "/akoflow-api/execution-runs/run/":
			value = map[string]any{"run": run, "handles": []any{map[string]any{"log": log}}}
			if r.URL.RawQuery != "" {
				value = map[string]any{"run": run}
			}
		case "/akoflow-api/schedule-plans/plan/context/":
			value = map[string]any{"workflow": workflow, "plan": map[string]any{"id": "plan", "workflowVersionId": "version"}}
		case "/akoflow-api/execution-runs/run/activity-metrics/":
			value = map[string]any{"items": []any{}}
		default:
			t.Errorf("unexpected source request: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(value)
	})
}
