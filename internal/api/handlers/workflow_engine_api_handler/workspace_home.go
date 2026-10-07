package workflow_engine_api_handler

import (
	"context"
	"fmt"
	"path"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

// Resolve execution paths from the discovered catalog, without changing the
// frozen placement or assuming that compute-node discovery includes login facts.
func (h *Handler) resolveWorkspaceHomes(ctx context.Context, request *ports.ExecutionRequest) error {
	connections, ok := h.environments.(ports.ConnectionStore)
	if !ok {
		return nil
	}
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
		isSlurm := false
		for _, runtime := range definition.Runtimes {
			if runtime.Driver == domain.RuntimeDriverSlurm {
				isSlurm = true
			}
		}
		if !isSlurm {
			continue
		}
		home := ""
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
					return fmt.Errorf("ambiguous discovered home for connection %q", connectionID)
				}
				home = candidate
			}
		}
		if home != "" {
			metadata := make(map[string]any, len(resource.Metadata)+1)
			for key, value := range resource.Metadata {
				metadata[key] = value
			}
			metadata["homeDirectory"] = home
			resource.Metadata = metadata
		}
	}
	return nil
}
