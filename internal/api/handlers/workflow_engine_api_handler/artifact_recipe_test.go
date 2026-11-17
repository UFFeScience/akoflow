package workflow_engine_api_handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	appbuild "github.com/UFFeScience/akoflow/internal/application/build"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestRecipeRegistrationPersistsSourceWithoutBuildAndCanInspect(t *testing.T) {
	catalog := freshOriginCatalog(t)
	h := newTestHandler()
	h.data = catalog
	h.build = appbuild.Manager{Root: t.TempDir(), Catalog: catalog}
	var spec domain.ArtifactBuild
	var origin domain.ArtifactOrigin
	for attempt := 0; attempt < 2; attempt++ {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("dockerfile", "FROM scratch\n")
		_ = writer.WriteField("platform", "linux/amd64")
		_ = writer.Close()
		request := httptest.NewRequest(http.MethodPost, "/", &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		response := httptest.NewRecorder()
		h.RegisterArtifactRecipe(response, request)
		if response.Code != http.StatusCreated && response.Code != http.StatusOK {
			t.Fatalf("recipe registration: %d %s", response.Code, response.Body.String())
		}
		var result struct {
			Origin domain.ArtifactOrigin `json:"origin"`
			Build  domain.ArtifactBuild  `json:"build"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if attempt > 0 && result.Build.ID != spec.ID {
			t.Fatal("duplicate recipe created another build")
		}
		spec, origin = result.Build, result.Origin
	}
	stored, err := catalog.FindArtifactBuild(t.Context(), spec.ID)
	if err != nil || stored == nil || stored.TargetFormat != "oci" {
		t.Fatalf("source specification not persisted: %+v %v", stored, err)
	}
	runs, err := catalog.ListBuildRuns(t.Context(), spec.ID)
	if err != nil || len(runs) != 0 {
		t.Fatalf("registration started a build: %+v %v", runs, err)
	}
	payload, _ := json.Marshal(domain.ExecutableSource{Type: domain.ExecutableSourceBuild, ArtifactBuildRef: spec.ID, Origin: &origin})
	response := callHandler(t, http.MethodPost, "/", string(payload), nil, h.InspectArtifactImage)
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte("FROM scratch")) || !bytes.Contains(response.Body.Bytes(), []byte(`"available":false`)) {
		t.Fatalf("source inspection: %d %s", response.Code, response.Body.String())
	}
}
