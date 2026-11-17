package hubpublish

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPublicationManagementPreservesPartialPatchAndInvalidatesReviews(t *testing.T) {
	patched, deleted := false, false
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing scoped connection")
		}
		switch {
		case r.URL.Path == "/api/v1/desktop/me":
			json.NewEncoder(w).Encode(Identity{Username: "researcher", ProjectID: "project"})
		case r.Method == "PATCH":
			var fields map[string]any
			json.NewDecoder(r.Body).Decode(&fields)
			if len(fields) != 1 || fields["title"] != "Updated" {
				t.Errorf("partial patch changed omitted fields: %v", fields)
			}
			patched = true
			w.WriteHeader(204)
		case r.Method == "DELETE":
			deleted = true
			w.WriteHeader(204)
		default:
			json.NewEncoder(w).Encode(map[string]any{"publication": map[string]string{"id": "publication", "title": "Updated"}})
		}
	})
	manager.state.AccessToken = "secret"
	manager.state.DefaultPublicationID = "publication"
	manager.previews["pending"] = preparedPublication{Publication: Publication{ID: "publication"}}
	title := "Updated"
	if _, err := manager.UpdatePublication(context.Background(), "publication", PublicationPatch{Title: &title}); err != nil {
		t.Fatal(err)
	}
	if !patched || len(manager.previews) != 0 {
		t.Fatal("update retained stale reviews")
	}
	if err := manager.DeletePublication(context.Background(), "publication"); err != nil {
		t.Fatal(err)
	}
	if !deleted || manager.state.DefaultPublicationID != "" {
		t.Fatal("delete retained default destination")
	}
}

func TestPublicationManagementRequiresAcknowledgementAndRespectsReadOnly(t *testing.T) {
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) { t.Error("rejected mutation contacted Hub") })
	public := "public"
	if _, err := manager.UpdatePublication(context.Background(), "publication", PublicationPatch{Visibility: &public}); err == nil {
		t.Fatal("public visibility did not require acknowledgement")
	}
	manager.options.ReadOnly = true
	title := "title"
	if _, err := manager.UpdatePublication(context.Background(), "publication", PublicationPatch{Title: &title}); err == nil {
		t.Fatal("read-only update accepted")
	}
	if err := manager.DeletePublication(context.Background(), "publication"); err == nil {
		t.Fatal("read-only delete accepted")
	}
	if _, err := manager.ImportWorkflow(context.Background(), definitionURI, ""); err == nil {
		t.Fatal("read-only import accepted")
	}
}

func TestWorkflowVersionConfirmationUsesReviewedSnapshot(t *testing.T) {
	reviewed := "name: original\nspec: {}\n"
	published := false
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/desktop/me":
			json.NewEncoder(w).Encode(Identity{Username: "researcher", ProjectID: "project"})
		case "/api/v1/desktop/publications/publication":
			json.NewEncoder(w).Encode(publicationInfo{Publication: Publication{ID: "publication", Visibility: "public", Owner: "researcher", Slug: "study"}, Versions: []publicationVersion{{Number: 2}}})
		case "/api/v1/desktop/publications/publication/versions/preview":
			json.NewEncoder(w).Encode(map[string]string{"diff": "review"})
		case "/api/v1/desktop/publications/publication/versions":
			var input publicationVersionInput
			json.NewDecoder(r.Body).Decode(&input)
			if input.YAML != "name: original\nspec: {}\n" || input.BaseVersion != 2 {
				t.Errorf("published unreviewed content: %+v", input)
			}
			published = true
			json.NewEncoder(w).Encode(publicationVersion{Number: 3})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	manager.state.AccessToken = "secret"
	manager.options.Source = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/akoflow-api/workflow-definitions/workflow/export/" {
			t.Error(r.URL.Path)
		}
		w.Write([]byte(reviewed))
	})
	preview, err := manager.PreviewWorkflow(context.Background(), "workflow", "publication", "Update")
	if err != nil {
		t.Fatal(err)
	}
	reviewed = "name: edited-after-review\n"
	if _, err = manager.Confirm(context.Background(), preview.ID, false); err == nil || published {
		t.Fatal("public version bypassed acknowledgement")
	}
	receipt, err := manager.Confirm(context.Background(), preview.ID, true)
	if err != nil || !published || receipt.Version != 3 {
		t.Fatalf("confirmation: %+v %v", receipt, err)
	}
}

func TestHubImportPreservesImageMetadataAndDoesNotOverwriteConflicts(t *testing.T) {
	yaml := "name: portable\nspec:\n  inspection:\n    schemaVersion: 1\n    data: {format: sif, Id: sha256:example}\n"
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(yaml)) })
	conflict := false
	manager.options.Source = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.Query().Get("name") != "Imported study" || string(body) != yaml {
			t.Error("import changed source metadata or destination")
		}
		if conflict {
			w.WriteHeader(409)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"imported"}`))
	})
	value, err := manager.ImportWorkflow(context.Background(), definitionURI, "Imported study")
	if err != nil || !strings.Contains(string(value), "imported") {
		t.Fatalf("import %s %v", value, err)
	}
	conflict = true
	if _, err = manager.ImportWorkflow(context.Background(), definitionURI, "Imported study"); err == nil {
		t.Fatal("conflict silently overwritten")
	}
}
