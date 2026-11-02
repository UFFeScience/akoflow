package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRemoveOnlyHPCCapAndPreserveDecision(t *testing.T) {
	hpc := map[string]any{"type": "hpc_machine", "metadata": map[string]any{"workspaceStorageBudget": 123, "runtime": "sif"}}
	cloud := map[string]any{"type": "cloud_vm", "metadata": map[string]any{"workspaceStorageBudget": 456}}
	r := map[string]any{"Run": map[string]any{"mode": "simulation"}, "Resources": []any{hpc, cloud}}
	n, err := removeCap(r, "2026-10-11")
	require.NoError(t, err)
	require.Equal(t, 1, n)
	m := hpc["metadata"].(map[string]any)
	require.NotContains(t, m, "workspaceStorageBudget")
	require.Equal(t, "sif", m["runtime"])
	require.Equal(t, 123, m["workspaceStorageBudgetDecision"].(map[string]any)["previousBudget"])
	require.Equal(t, 456, cloud["metadata"].(map[string]any)["workspaceStorageBudget"])
}

func TestRejectReal(t *testing.T) {
	_, err := removeCap(map[string]any{"Run": map[string]any{"mode": "real"}}, "now")
	require.ErrorContains(t, err, "simulation-only")
}
