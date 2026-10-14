package workflow_engine_api_handler

import (
	"fmt"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

// Only preflight uses these copies. The persisted execution retains the live
// quota observation and explicit user risk acceptance. OS quota stays enforced.
func preflightStorageResources(request ports.ExecutionRequest) ([]domain.Resource, error) {
	resources := append([]domain.Resource(nil), request.Resources...)
	for i := range resources {
		r := &resources[i]
		raw := r.Metadata["workspaceStorageQuotaOverride"]
		if raw == nil {
			continue
		}
		o, ok := raw.(map[string]any)
		if !ok || r.Type != domain.ResourceHPCMachine || o["runId"] != request.Run.ID || request.Run.ID == "" || o["acceptedRisk"] != true || o["source"] != "explicit-user-authorization" || o["reason"] == "" || o["reason"] == nil {
			return nil, fmt.Errorf("invalid or unscoped HPC quota preflight override for %s", r.ID)
		}
		m := map[string]any{}
		for k, v := range r.Metadata {
			m[k] = v
		}
		delete(m, "workspaceStorageBudget")
		r.Metadata = m
	}
	return resources, nil
}
