package workflow_engine_api_handler

import (
	"context"
	"fmt"
	"path"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/storagebudget"
)

// Resolve execution paths from the discovered catalog, without changing the
// frozen placement or assuming that compute-node discovery includes login facts.
func (h *Handler) resolveWorkspaceHomes(ctx context.Context, request *ports.ExecutionRequest) error {
	connections, ok := h.environments.(ports.ConnectionStore)
	if !ok {
		return nil
	}
	needsBudget := false
	for _, activity := range request.Workflow.Activities {
		needsBudget = needsBudget || activity.Metadata["workspaceProjection"] != nil
	}
	observer, canObserve := h.storage.(interface {
		WorkspaceBudget(context.Context, string, string) (storagebudget.Snapshot, error)
	})
	budgets := map[string]storagebudget.Snapshot{}
	for i := range request.Resources {
		resource := &request.Resources[i]
		connectionID, _ := resource.Metadata["connectionId"].(string)
		if connectionID == "" {
			continue
		}
		connection, err := connections.FindConnection(ctx, connectionID)
		if err != nil {
			return err
		}
		if connection == nil {
			continue
		}
		definition, err := h.environments.Find(ctx, connection.EnvironmentID)
		if err != nil {
			return err
		}
		if definition == nil || definition.Version.ID != resource.EnvironmentVersionID {
			continue
		}
		home, homeStorageID, err := discoveredWorkspaceHome(definition, connectionID)
		if err != nil {
			return err
		}
		if selected, _ := resource.Metadata["workspaceStorageId"].(string); selected != "" {
			home, homeStorageID, err = selectedWorkspaceRoot(definition, connectionID, selected)
			if err != nil {
				return err
			}
		}
		if home != "" {
			metadata := make(map[string]any, len(resource.Metadata)+1)
			for key, value := range resource.Metadata {
				metadata[key] = value
			}
			metadata["homeDirectory"] = home
			if needsBudget && canObserve {
				budget, exists := budgets[homeStorageID]
				if !exists {
					budget, err = observer.WorkspaceBudget(ctx, homeStorageID, "")
					if err != nil {
						return fmt.Errorf("discover workspace quota: %w", err)
					}
					budgets[homeStorageID] = budget
				}
				metadata["workspaceStorageBudget"] = budget
			}
			resource.Metadata = metadata
		}
	}
	return nil
}

// An explicit workspace is a catalog storage owned by the same connection.
// Its quota is observed normally; selecting scratch never disables preflight.
func selectedWorkspaceRoot(definition *domain.EnvironmentDefinition, connectionID, storageID string) (string, string, error) {
	for _, storage := range definition.Storages {
		if storage.ID != storageID {
			continue
		}
		owner, _ := storage.Metadata["connectionId"].(string)
		if owner != connectionID || storage.ReadOnly || !path.IsAbs(storage.Endpoint) || path.Clean(storage.Endpoint) == "/" {
			return "", "", fmt.Errorf("workspace storage %q is not a writable absolute root of connection %q", storageID, connectionID)
		}
		return storage.Endpoint, storage.ID, nil
	}
	return "", "", fmt.Errorf("workspace storage %q unavailable in the resource environment", storageID)
}

// A home is eligible only when the same catalog connection discovers a Slurm
// runtime and an absolute home browse root matching its storage endpoint.
func discoveredWorkspaceHome(definition *domain.EnvironmentDefinition, connectionID string) (string, string, error) {
	isSlurm := false
	for _, runtime := range definition.Runtimes {
		isSlurm = isSlurm || runtime.Driver == domain.RuntimeDriverSlurm
	}
	if !isSlurm {
		return "", "", nil
	}
	home, storageID := "", ""
	for _, storage := range definition.Storages {
		id, _ := storage.Metadata["connectionId"].(string)
		if id != connectionID {
			continue
		}
		roots, _ := storage.Configuration["browseRoots"].([]any)
		for _, raw := range roots {
			root, _ := raw.(map[string]any)
			label, _ := root["label"].(string)
			candidate, _ := root["path"].(string)
			if label != "home" || !path.IsAbs(candidate) || candidate == "/" || candidate != storage.Endpoint {
				continue
			}
			if home != "" && home != candidate {
				return "", "", fmt.Errorf("ambiguous discovered home for connection %q", connectionID)
			}
			home, storageID = candidate, storage.ID
		}
	}
	return home, storageID, nil
}
