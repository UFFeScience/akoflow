package workflow_engine_api_handler

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestResolveWorkspaceHomesUsesExactDiscoveredConnection(t *testing.T) {
	for _, connectionID := range []string{"ssh", "other"} {
		t.Run(connectionID, func(t *testing.T) {
			definition := domain.EnvironmentDefinition{
				Version:  domain.EnvironmentVersion{ID: "v1"},
				Runtimes: []domain.EnvironmentRuntime{{Driver: domain.RuntimeDriverSlurm}},
				Storages: []domain.StorageResource{{Endpoint: "/home/scientist", Metadata: map[string]any{"connectionId": connectionID}, Configuration: map[string]any{"browseRoots": []any{map[string]any{"label": "home", "path": "/home/scientist"}}}}},
			}
			h := Handler{environments: &environmentCatalogRichStub{found: &definition, connection: &domain.EnvironmentConnection{ID: "ssh", EnvironmentID: "hpc"}}}
			original := map[string]any{"connectionId": "ssh"}
			request := ports.ExecutionRequest{Resources: []domain.Resource{{ID: "node", EnvironmentVersionID: "v1", Metadata: original}}}
			if err := h.resolveWorkspaceHomes(context.Background(), &request); err != nil {
				t.Fatal(err)
			}
			home, _ := request.Resources[0].Metadata["homeDirectory"].(string)
			if connectionID == "ssh" && home != "/home/scientist" {
				t.Fatalf("home=%q", home)
			}
			if connectionID == "other" && home != "" {
				t.Fatalf("foreign connection home=%q", home)
			}
			if _, ok := original["homeDirectory"]; ok {
				t.Fatal("mutated frozen input metadata")
			}
		})
	}
}
