package hubpublish

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestProfileExecutionReviewSharesOnlyProjectedData(t *testing.T) {
	const runID = "private-run"
	var submitted []profileExecutionInput
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/desktop/me":
			_, _ = io.WriteString(w, `{"username":"researcher"}`)
		case "/api/v1/desktop/profile-executions":
			if r.Header.Get("Authorization") != "Bearer secret" {
				t.Error("share request missing authentication")
			}
			body, _ := io.ReadAll(r.Body)
			for _, private := range []string{runID, "private-host", "private-workflow", "private-command", "private-input", "private-alias"} {
				if strings.Contains(string(body), private) {
					t.Errorf("share exposed %s: %s", private, body)
				}
			}
			var value profileExecutionInput
			if err := json.Unmarshal(body, &value); err != nil {
				t.Error(err)
			}
			submitted = append(submitted, value)
			_, _ = io.WriteString(w, `{"id":"public-id","author":"researcher"}`)
		default:
			t.Errorf("unexpected Hub request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	manager.state.AccessToken = "secret"
	manager.options.Source = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/akoflow-api/execution-runs/private-run/" || r.URL.Query().Get("view") != "monitor" {
			t.Errorf("unexpected daemon snapshot request %s", r.URL)
		}
		_, _ = io.WriteString(w, `{
			"run":{"id":"private-run","status":"completed","mode":"real","finishedAt":"2026-11-17T23:30:00-03:00","makespanSeconds":10,"workflowVersionId":"private-workflow"},
			"activities":[{"activityId":"private-command","startedAt":1,"allocatedResourceId":"private-alias"}],
			"handles":[{"id":"private-run:private-command","metadata":{"allocatedNode":"private-host","slurmEnvironment":{"architecture":"amd64","os":"linux","cpuCores":8}}}],
			"files":[{"path":"private-input"}]
		}`)
	})
	ctx := context.Background()
	review, err := manager.previewProfileExecution(ctx, runID, "Lab workstation")
	if err != nil {
		t.Fatal(err)
	}
	if review.Author != "researcher" || review.Visibility != "public" || review.Summary.Date != "2026-11-18" ||
		len(review.Summary.Resources) != 1 || review.Summary.Resources[0].Runtime != "Slurm" || len(submitted) != 0 {
		t.Fatalf("unexpected unconfirmed review: %+v", review)
	}
	if _, err := manager.Confirm(ctx, review.ID, false); err == nil || len(submitted) != 0 {
		t.Fatal("shared without public confirmation")
	}
	receipt, err := manager.Confirm(ctx, review.ID, true)
	if err != nil || !strings.HasSuffix(receipt.URL, "/u/researcher/executions/public-id") {
		t.Fatalf("unexpected share receipt: %+v, %v", receipt, err)
	}
	next, err := manager.previewProfileExecution(ctx, runID, "Lab workstation")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Confirm(ctx, next.ID, true); err != nil || len(submitted) != 2 ||
		len(submitted[0].ShareKey) != 64 || submitted[0].ShareKey != submitted[1].ShareKey {
		t.Fatalf("sharing the same local run should reuse its opaque key: %+v, %v", submitted, err)
	}
}
