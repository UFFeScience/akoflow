package workflow_engine_api_handler

import (
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestQuotaOverrideScopedAndKeepsObservedSnapshot(t *testing.T) {
	m := map[string]any{"workspaceStorageBudget": "live", "workspaceStorageQuotaOverride": map[string]any{"runId": "run", "acceptedRisk": true, "source": "explicit-user-authorization", "reason": "rolling cleanup experiment"}}
	r := ports.ExecutionRequest{Run: domain.ExecutionRun{ID: "run"}, Resources: []domain.Resource{{ID: "hpc", Type: domain.ResourceHPCMachine, Metadata: m}}}
	out, err := preflightStorageResources(r)
	require.NoError(t, err)
	require.NotContains(t, out[0].Metadata, "workspaceStorageBudget")
	require.Equal(t, "live", m["workspaceStorageBudget"])
	r.Run.ID = "different"
	_, err = preflightStorageResources(r)
	require.Error(t, err)
	r.Run.ID = "run"
	r.Resources[0].Type = domain.ResourceCloudVM
	_, err = preflightStorageResources(r)
	require.Error(t, err)
}
