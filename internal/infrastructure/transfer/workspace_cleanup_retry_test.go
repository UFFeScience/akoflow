package transfer

import (
	"os"
	"path/filepath"
	"testing"
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
