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
)

func TestDockerArtifactDocumentationRequestRegistersBuildSpecification(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(filepath.Join(t.TempDir(), "artifacts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Bootstrap(ctx, db); err != nil {
		t.Fatal(err)
	}
	store := datarepo.New(db)
	handler := &Handler{data: store}
	request := httptest.NewRequest(http.MethodPost, "/artifacts/docker/", strings.NewReader(`{"artifactId":"busybox","version":"1.36","image":"docker.io/library/busybox:1.36","architecture":"amd64"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.RegisterDockerArtifact(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("register Docker artifact: %d %s", recorder.Code, recorder.Body.String())
	}
	var result struct {
		Artifact domain.ArtifactVersion `json:"artifact"`
		Build    domain.ArtifactBuild   `json:"build"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	stored, err := store.FindArtifactBuild(ctx, result.Build.ID)
	if err != nil || stored == nil || result.Artifact.ArtifactID != "busybox" || stored.RecipePath != "docker.io/library/busybox:1.36" || stored.TargetArchitecture != "amd64" || stored.SourceType != "docker-image" {
		t.Fatalf("stored Docker build specification: response=%#v stored=%#v error=%v", result, stored, err)
	}
}
