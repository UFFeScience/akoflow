package hubpublish

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestNamedReferencesResolveLatestOnEachLoadAndPinSpecificVersions(t *testing.T) {
	latest := 1
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workflow-definitions/resolve" {
			t.Fatal("unexpected fetch", r.URL.Path)
		}
		reference := r.URL.Query().Get("reference")
		version := latest
		if strings.HasSuffix(reference, ":v1") {
			version = 1
		}
		w.Header().Set("X-Akoflow-Workflow-Version", fmt.Sprint(version))
		fmt.Fprintf(w, "name: study-version-%d\n", version)
	})
	first, err := manager.WorkflowDefinition(context.Background(), "akoflow::ovvesley/story-signal-study:latest")
	if err != nil || first.Version != 1 {
		t.Fatal(err, first)
	}
	latest = 2
	second, err := manager.WorkflowDefinition(context.Background(), "akoflow::ovvesley/story-signal-study:latest")
	if err != nil || second.Version != 2 || second.URI != "akoflow::ovvesley/story-signal-study:v2" {
		t.Fatal(err, second)
	}
	fixed, err := manager.WorkflowDefinition(context.Background(), "/u/ovvesley/story-signal-study:v1")
	if err != nil || fixed.Version != 1 || fixed.YAML != first.YAML {
		t.Fatal(err, fixed)
	}
}
func TestNamedReferencesRejectOriginsAndInvalidTagsBeforeFetching(t *testing.T) {
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected network request") })
	for _, reference := range []string{
		"//evil.example/u/owner/workflow:latest",
		"/u/owner/workflow:latest?origin=https://evil.example",
		"/u/owner/workflow:v0",
		"/u/owner/../workflow:latest",
	} {
		if _, err := manager.WorkflowDefinition(context.Background(), reference); err == nil {
			t.Fatal("accepted", reference)
		}
	}
}
