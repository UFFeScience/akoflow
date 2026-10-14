package workflow_engine_api_handler

import (
	"context"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/storagebudget"
	"github.com/stretchr/testify/require"
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

type budgetNavigatorStub struct {
	storageNavigatorStub
	calls int
}

func (s *budgetNavigatorStub) WorkspaceBudget(_ context.Context, id, path string) (storagebudget.Snapshot, error) {
	s.calls++
	return storagebudget.Snapshot{StorageID: "shared-user-quota", AvailableBytes: 180 * 1024, Source: "live-posix-filesystem-and-user-quota-v1", ObservedAt: time.Now().UTC()}, nil
}

func TestResolveWorkspaceBudgetRefreshesAndSharesExactObservation(t *testing.T) {
	definition := domain.EnvironmentDefinition{
		Version:  domain.EnvironmentVersion{ID: "v1"},
		Runtimes: []domain.EnvironmentRuntime{{Driver: domain.RuntimeDriverSlurm}},
		Storages: []domain.StorageResource{
			{ID: "home", Endpoint: "/home/scientist", Metadata: map[string]any{"connectionId": "ssh"}, Configuration: map[string]any{"browseRoots": []any{map[string]any{"label": "home", "path": "/home/scientist"}}}},
		},
	}
	navigator := &budgetNavigatorStub{}
	h := Handler{storage: navigator, environments: &environmentCatalogRichStub{found: &definition, connection: &domain.EnvironmentConnection{ID: "ssh", EnvironmentID: "hpc"}}}
	request := ports.ExecutionRequest{
		Workflow: domain.WorkflowVersion{Activities: []domain.Activity{{Metadata: map[string]any{"workspaceProjection": true}}}},
		Resources: []domain.Resource{
			{ID: "node1", EnvironmentVersionID: "v1", Metadata: map[string]any{"connectionId": "ssh", "workspaceStorageBudget": "stale"}},
			{ID: "node2", EnvironmentVersionID: "v1", Metadata: map[string]any{"connectionId": "ssh"}},
		},
	}
	require.NoError(t, h.resolveWorkspaceHomes(context.Background(), &request))
	require.Equal(t, 1, navigator.calls)
	require.Equal(t, request.Resources[0].Metadata["workspaceStorageBudget"], request.Resources[1].Metadata["workspaceStorageBudget"])
	require.Equal(t, int64(180*1024), request.Resources[0].Metadata["workspaceStorageBudget"].(storagebudget.Snapshot).AvailableBytes)
}
