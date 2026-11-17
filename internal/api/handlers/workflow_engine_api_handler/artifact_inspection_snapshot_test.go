package workflow_engine_api_handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appbuild "github.com/UFFeScience/akoflow/internal/application/build"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
	"github.com/UFFeScience/akoflow/internal/hubpublish"
)

func TestPublishedInspectionRoundTripsAndRemainsReadableWithoutImage(t *testing.T) {
	for _, format := range []string{"oci", "sif"} {
		t.Run(format, func(t *testing.T) {
			data := map[string]any{"format": format, "reference": "published-image", "Id": "sha256:" + strings.Repeat("a", 64)}
			source := domain.ExecutableSource{Type: domain.ExecutableSourceOCI, Reference: "ghcr.io/example/tool@sha256:" + strings.Repeat("b", 64)}
			if format == "sif" {
				source.Type = domainworkflow.ExecutableSourceLocalFile
				source.Path = "/missing/tool.sif"
				source.Format = domain.ExecutableFormatSIF
			}
			data["reference"] = source.Reference
			if format == "sif" {
				data["reference"] = source.Path
			}
			data["platform"] = "linux/amd64"
			data["history"] = []any{map[string]any{"CreatedBy": "RUN echo example", "Size": "1kB"}}
			data["sifPartitions"] = "1 |1 |NONE |100-200 |Def.FILE"
			data["sifMetadata"] = map[string]any{"data": map[string]any{"attributes": map[string]any{"deffile": "Bootstrap: docker\nFrom: alpine"}}}
			source.Inspection = &domainworkflow.ArtifactInspection{SchemaVersion: 1, Data: data}
			h := newTestHandler()
			h.build = appbuild.Manager{Root: t.TempDir()}
			definition := workflowFixture()
			definition.Version.Activities[0].Command.Executable = &domain.ExecutableReference{Source: source}
			h.workflows = &workflowRepositoryStub{definition: &definition}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/export", nil)
			request.SetPathValue("workflowId", definition.ID)
			h.ExportWorkflow(response, request)
			if response.Code != 200 {
				t.Fatalf("export: %d %s", response.Code, response.Body.String())
			}
			imported := newTestHandler()
			imported.workflows = &workflowRepositoryStub{}
			request = httptest.NewRequest(http.MethodPost, "/import", strings.NewReader(response.Body.String()))
			request.Header.Set("Content-Type", "application/yaml")
			response = httptest.NewRecorder()
			imported.CreateWorkflow(response, request)
			if response.Code != 201 {
				t.Fatalf("import: %d %s", response.Code, response.Body.String())
			}
			restored := imported.workflows.(*workflowRepositoryStub).created.Version.Activities[0].Command.Executable.Source
			imported.build = appbuild.Manager{Root: t.TempDir()}
			metadata, err := imported.inspectArtifactSource(context.Background(), restored)
			if err != nil || metadata["metadataProvenance"] != "published-snapshot" || metadata["available"] != false || metadata["Id"] != data["Id"] {
				t.Fatalf("imported inspection unavailable: %+v %v", metadata, err)
			}
			original, _ := json.Marshal(source.Inspection.Data)
			roundtrip, _ := json.Marshal(restored.Inspection.Data)
			if string(original) != string(roundtrip) {
				t.Fatal("image metadata changed across export/import")
			}
		})
	}
}

func TestInspectionPublicationRemovesEmbeddedCredentials(t *testing.T) {
	source := domain.ExecutableSource{Type: domain.ExecutableSourceOCI, Reference: "registry/image:1"}
	data := map[string]any{"format": "oci", "Config": map[string]any{"Env": []any{"API_TOKEN=private-value", "PATH=/bin"}, "password": "private-password"}}
	cleaned, err := hubpublish.SanitizeArtifactInspection(data)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(cleaned)
	if strings.Contains(string(encoded), "private-value") || strings.Contains(string(encoded), "private-password") {
		t.Fatal("inspection exported credentials")
	}
	source.Inspection = &domainworkflow.ArtifactInspection{SchemaVersion: 1, Data: cleaned}
	if err := source.Inspection.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRepeatedRecipeExportDoesNotCreateTimestampOnlyPublicationChanges(t *testing.T) {
	source, definition := sourceOriginFixture(t, "docker.io/library/ubuntu@sha256:"+strings.Repeat("a", 64), true)
	first, err := source.portableDefinition(context.Background(), *definition)
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.portableDefinition(context.Background(), *definition)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := json.Marshal(first)
	right, _ := json.Marshal(second)
	if string(left) != string(right) {
		t.Fatal("identical artifact export is not deterministic")
	}
}
