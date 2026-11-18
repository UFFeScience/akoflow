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
	const publicationID = "3f6f378d-dcb8-4a28-883e-2b26697c0393"
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
		w.Header().Set("X-Akoflow-Publication-ID", publicationID)
		fmt.Fprintf(w, "name: study-version-%d\n", version)
	})
	first, err := manager.WorkflowDefinition(context.Background(), "akoflow::ovvesley/story-signal-study:latest")
	if err != nil || first.Version != 1 || first.PublicationID != publicationID {
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

func TestAnonymousReferencesKeepWorkflowNameAndPinVersion(t *testing.T) {
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("reference") != "/a/3f6f378d-dcb8-4a28-883e-2b26697c0393/montage-50d:latest" {
			t.Fatal("anonymous URI changed", r.URL.RawQuery)
		}
		w.Header().Set("X-Akoflow-Workflow-Version", "2")
		fmt.Fprint(w, "name: montage-50d\n")
	})
	result, err := manager.WorkflowDefinition(context.Background(), "akoflow::a/3f6f378d-dcb8-4a28-883e-2b26697c0393/montage-50d:latest")
	if err != nil || result.URI != "akoflow::a/3f6f378d-dcb8-4a28-883e-2b26697c0393/montage-50d:v2" {
		t.Fatal(result, err)
	}
}

func TestConfiguredBrowserWorkflowLinks(t *testing.T) {
	const origin = "http://127.0.0.1:8080"
	const browser = "https://hub.example.org"
	for input, expected := range map[string]string{
		browser + "/u/researcher/study":                     "/u/researcher/study:latest",
		browser + "/o/research-team/study/versions/12/code": "/o/research-team/study:v12",
		browser + "/a/anonymous-owner/study#versions/v3":    "/a/anonymous-owner/study:v3",
		origin + "/u/researcher/study/versions/5":           "/u/researcher/study:v5",
	} {
		result, err := browserWorkflowReference(input, origin, browser)
		if err != nil || result != expected {
			t.Errorf("%s: %q, %v", input, result, err)
		}
	}
	for _, input := range []string{
		"https://hub.example.org.invalid/u/researcher/study",
		"https://user@hub.example.org/u/researcher/study",
		browser + "/u/researcher/study/versions/0",
		browser + "/u/researcher/study/versions/2/unknown",
		browser + "/api/v1/publications/secrets",
	} {
		if _, err := browserWorkflowReference(input, origin, browser); err == nil {
			t.Errorf("accepted unsupported link %s", input)
		}
	}
}

func TestImportRejectsDefinitionChangedSinceReview(t *testing.T) {
	manager := testManager(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Akoflow-Workflow-Version", "2")
		fmt.Fprint(w, "name: changed")
	})
	manager.options.Source = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the local import must not run when reviewed content changed")
	})
	_, err := manager.importWorkflow(context.Background(), "akoflow::reviewer/study:v2", "copy", "old-hash", "name: edited")
	fault, ok := err.(*apiError)
	if !ok || fault.Code != "workflow_source_changed" {
		t.Fatalf("expected changed source error, got %v", err)
	}
}
