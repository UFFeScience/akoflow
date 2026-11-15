package hubpublish

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const definitionURI = "akoflow://hub/publications/35746b14-7dc2-454c-a74f-902d58b50c94/versions/1"

func TestWorkflowURIRejectsOriginsAndMutableOrMalformedReferences(t *testing.T) {
	for _, uri := range []string{
		"https://example.org/workflow.yaml",
		"akoflow://evil/publications/35746b14-7dc2-454c-a74f-902d58b50c94/versions/1",
		definitionURI + "?url=http://169.254.169.254",
		definitionURI + "#token",
		strings.Replace(definitionURI, "/1", "/latest", 1),
		strings.Replace(definitionURI, "/1", "/0", 1),
		"akoflow://hub/publications/invalid/versions/1",
	} {
		if _, _, err := parseWorkflowURI(uri); err == nil {
			t.Fatalf("accepted unsafe URI: %s", uri)
		}
	}
}
func TestWorkflowDefinitionFetchesOnlyYAMLWithoutBrowserCredentials(t *testing.T) {
	requests := 0
	yaml := "name: study\nspec:\n  activities: []\n"
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/api/v1/publications/35746b14-7dc2-454c-a74f-902d58b50c94/versions/1/yaml" || r.Method != "GET" {
			t.Error("not an exact version YAML read", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("public read leaked connection credentials")
		}
		w.Write([]byte(yaml))
	})
	manager.state.AccessToken = "private-connection-token"
	definition, err := manager.WorkflowDefinition(context.Background(), definitionURI)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(definition)
	if requests != 1 || definition.YAML != yaml || len(definition.Hash) != 64 || strings.Contains(string(raw), manager.state.AccessToken) {
		t.Fatalf("invalid definition response: %s", raw)
	}
}
func TestWorkflowDefinitionUsesScopedPrivateReadAndNeverFollowsRedirects(t *testing.T) {
	calls := []string{}
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		if !strings.Contains(r.URL.Path, "/desktop/") {
			w.WriteHeader(404)
			return
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing server-only credentials")
		}
		w.Write([]byte("name: private-study\n"))
	})
	manager.state.AccessToken = "secret"
	if _, err := manager.WorkflowDefinition(context.Background(), definitionURI); err != nil || len(calls) != 2 {
		t.Fatal(err, calls)
	}
	reached := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer target.Close()
	redirect := testManager(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) })
	if _, err := redirect.WorkflowDefinition(context.Background(), definitionURI); err == nil || reached {
		t.Fatal("redirect was followed")
	}
}
