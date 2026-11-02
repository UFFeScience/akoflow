package workflow_engine_api_handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	datarepo "github.com/UFFeScience/akoflow/internal/infrastructure/database/data"
	"github.com/UFFeScience/akoflow/internal/infrastructure/dockerhub"
)

func TestDockerHubImportValidatesSelectionBeforeRegistrationAndPinsBuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/missing") {
			w.WriteHeader(404)
			return
		}
		_, _ = w.Write([]byte(`{"images":[{"os":"linux","architecture":"amd64","digest":"sha256:` + strings.Repeat("a", 64) + `"}]}`))
	}))
	defer server.Close()
	client := dockerhub.New()
	client.BaseURL = server.URL
	db, err := database.Open(filepath.Join(t.TempDir(), "hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err = database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	store := datarepo.New(db)
	handler := &Handler{data: store, dockerHub: client}
	for _, test := range []struct {
		tag    string
		status int
	}{{"missing", 404}, {"3.12", 201}} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/", strings.NewReader(`{"artifactId":"python","version":"3.12","architecture":"amd64","dockerHub":{"repository":"python","tag":"`+test.tag+`"}}`))
		request.Header.Set("Content-Type", "application/json")
		handler.RegisterDockerArtifact(recorder, request)
		if recorder.Code != test.status {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body)
		}
		artifacts, err := store.ListArtifacts(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		if test.status == 404 && len(artifacts) != 0 {
			t.Fatal("invalid image registered")
		}
		if test.status == 201 {
			var result struct {
				Build domain.ArtifactBuild `json:"build"`
			}
			if err = json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Build.RecipePath != "docker.io/library/python@sha256:"+strings.Repeat("a", 64) || result.Build.TargetFormat != "sif" {
				t.Fatalf("build=%+v", result.Build)
			}
		}
	}
}
func TestDockerHubHandlersRejectInvalidPages(t *testing.T) {
	handler := &Handler{dockerHub: dockerhub.New()}
	for _, serve := range []http.HandlerFunc{handler.SearchDockerHub, handler.DockerHubTags} {
		recorder := httptest.NewRecorder()
		serve(recorder, httptest.NewRequest("GET", "/?page=-1", nil))
		if recorder.Code != 400 {
			t.Fatalf("status=%d", recorder.Code)
		}
	}
}
