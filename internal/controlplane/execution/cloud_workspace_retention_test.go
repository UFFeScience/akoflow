package execution

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestFailedRunRetainsExactProducerGenerationWithPendingConsumers(t *testing.T) {
	allocations := map[string]domain.RuntimeAllocation{
		"producer": {CloudInstanceID: "vm-new"},
		"neighbor": {CloudInstanceID: "vm-new"},
		"old":      {CloudInstanceID: "vm-old"},
	}
	workspaces := []domain.ActivityWorkspace{{ID: "workspace", State: domain.WorkspaceSealed, URI: "cloud://target/path?cloudInstanceId=vm-new"}}
	leases := []domain.WorkspaceLease{{WorkspaceID: "workspace"}}
	remaining, retained := retainCloudSourcesWithWorkspaceLeases(allocations, workspaces, leases)
	if retained != 1 || len(remaining) != 1 || remaining["old"].CloudInstanceID != "vm-old" {
		t.Fatalf("retained=%d allocations=%v", retained, remaining)
	}
	leases[0].Released = true
	remaining, retained = retainCloudSourcesWithWorkspaceLeases(allocations, workspaces, leases)
	if retained != 0 || len(remaining) != 3 {
		t.Fatal("released leases must allow lifecycle cleanup")
	}
}
