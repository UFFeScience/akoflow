package transfer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestLocalWorkspaceLifecycleRequiresOwnershipMarker(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "run-1", "activity-1")
	workspace := domain.ActivityWorkspace{
		ID: "workspace-run-1-activity-1", RunID: "run-1", ActivityID: "activity-1",
		ExecutionPath: path,
	}
	manager := ActivityWorkspaceManager{}
	if err := manager.Ensure(context.Background(), workspace); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "output.fits"), []byte("output"), 0o600); err != nil {
		t.Fatal(err)
	}
	usage, err := manager.Inspect(context.Background(), workspace)
	if err != nil || usage.FileCount != 1 || usage.SizeBytes != 6 {
		t.Fatalf("usage=%+v err=%v", usage, err)
	}
	result, err := manager.Release(context.Background(), workspace)
	if err != nil || result.ReclaimedBytes != 6 {
		t.Fatalf("release=%+v err=%v", result, err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("workspace still exists: %v", err)
	}
}

func TestLocalWorkspaceLifecycleRefusesWrongMarkerAndUnsafePath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "run-1", "activity-1")
	workspace := domain.ActivityWorkspace{
		ID: "workspace-run-1-activity-1", RunID: "run-1", ActivityID: "activity-1",
		ExecutionPath: path,
	}
	manager := ActivityWorkspaceManager{}
	if err := manager.Ensure(context.Background(), workspace); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workspaceMarker(path), []byte("someone else\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Release(context.Background(), workspace); err == nil {
		t.Fatal("workspace with mismatched marker was released")
	}
	workspace.ExecutionPath = root
	if _, err := manager.Release(context.Background(), workspace); err == nil {
		t.Fatal("workspace outside run/activity suffix was released")
	}
}
