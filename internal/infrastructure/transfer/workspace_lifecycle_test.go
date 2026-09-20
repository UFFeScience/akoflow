package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestPruneLocalWorkspaceInputsRemovesOnlyUnchangedInheritedFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "run-1", "activity-1")
	workspace := domain.ActivityWorkspace{ID: "workspace-run-1-activity-1", RunID: "run-1",
		ActivityID: "activity-1", ExecutionPath: path}
	manager := ActivityWorkspaceManager{}
	if err := manager.Ensure(context.Background(), workspace); err != nil {
		t.Fatal(err)
	}
	input := []byte("inherited")
	changed := []byte("changed-after-snapshot")
	if err := os.WriteFile(filepath.Join(path, "input.fits"), input, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "changed.fits"), changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "output.fits"), []byte("output"), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace.Manifest.Inputs = []domain.WorkspaceEntry{
		{Path: "input.fits", SizeBytes: int64(len(input)), Digest: "sha256:" + digestForTest(input)},
		{Path: "changed.fits", SizeBytes: 8, Digest: "sha256:" + digestForTest([]byte("original"))},
	}
	result, err := manager.PruneInputs(context.Background(), workspace)
	if err != nil {
		t.Fatal(err)
	}
	if result.RemovedFiles != 1 || result.PreservedFiles != 1 || result.ReclaimedBytes != int64(len(input)) {
		t.Fatalf("result=%+v", result)
	}
	if _, err := os.Stat(filepath.Join(path, "input.fits")); !os.IsNotExist(err) {
		t.Fatalf("unchanged input remains: %v", err)
	}
	for _, name := range []string{"changed.fits", "output.fits"} {
		if _, err := os.Stat(filepath.Join(path, name)); err != nil {
			t.Fatalf("%s was not preserved: %v", name, err)
		}
	}
}

func TestPruneLocalWorkspaceInputsRejectsEscapingPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "run-1", "activity-1")
	workspace := domain.ActivityWorkspace{ID: "workspace-run-1-activity-1", RunID: "run-1",
		ActivityID: "activity-1", ExecutionPath: path,
		Manifest: domain.WorkspaceManifest{Inputs: []domain.WorkspaceEntry{{Path: "../outside"}}}}
	manager := ActivityWorkspaceManager{}
	if err := manager.Ensure(context.Background(), workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.PruneInputs(context.Background(), workspace); err == nil {
		t.Fatal("unsafe input path was accepted")
	}
}

func digestForTest(value []byte) string {
	hash := sha256.Sum256(value)
	return hex.EncodeToString(hash[:])
}

func TestWorkspacePruneShellCommandRemovesOnlyVerifiedInputs(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "run-1", "activity-1")
	workspace := domain.ActivityWorkspace{ID: "workspace-run-1-activity-1", RunID: "run-1",
		ActivityID: "activity-1", ExecutionPath: path}
	manager := ActivityWorkspaceManager{}
	if err := manager.Ensure(context.Background(), workspace); err != nil {
		t.Fatal(err)
	}
	input := []byte("input")
	output := []byte("output")
	if err := os.WriteFile(filepath.Join(path, "input.fits"), input, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "output.fits"), output, 0o600); err != nil {
		t.Fatal(err)
	}
	workspace.Manifest.Inputs = []domain.WorkspaceEntry{{Path: "input.fits", SizeBytes: int64(len(input)), Digest: "sha256:" + digestForTest(input)}}
	command := exec.Command("sh", "-c", workspacePruneCommand(path, workspace))
	command.Stdin = strings.NewReader(workspacePrunePayload(workspace.Manifest.Inputs))
	resultJSON, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("shell pruning failed: %v: %s", err, resultJSON)
	}
	var result domain.WorkspaceReleaseResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		t.Fatal(err)
	}
	if result.RemovedFiles != 1 || result.ReclaimedBytes != int64(len(input)) {
		t.Fatalf("result=%+v", result)
	}
	if _, err := os.Stat(filepath.Join(path, "input.fits")); !os.IsNotExist(err) {
		t.Fatalf("input remains: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(path, "output.fits")); err != nil || string(content) != string(output) {
		t.Fatalf("output changed: %q %v", content, err)
	}
}
