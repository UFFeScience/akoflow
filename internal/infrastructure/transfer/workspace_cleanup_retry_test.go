package transfer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestOwnershipMarkerRemovalIsIdempotent(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "workspace.akoflow-owner")
	if err := os.WriteFile(marker, []byte("validated ownership"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := removeWorkspaceMarker(marker); err != nil {
		t.Fatal(err)
	}
	if err := removeWorkspaceMarker(marker); err != nil {
		t.Fatalf("concurrent releaser's prior removal must succeed: %v", err)
	}
}

func TestOnlyCloudDockerWorkspaceCleanupUsesPrivilege(t *testing.T) {
	for _, driver := range []string{"cloud-ssh", "ssh", "local", "kubernetes-pvc"} {
		command := workspaceRemovalCommand(domain.ActivityWorkspace{Driver: driver})
		if driver == "cloud-ssh" && command != "sudo -n rm" || driver != "cloud-ssh" && command != "rm" {
			t.Fatalf("driver=%s command=%s", driver, command)
		}
	}
}
