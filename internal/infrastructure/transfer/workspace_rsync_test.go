package transfer

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type workspaceEndpointResolver struct{}

func (workspaceEndpointResolver) ResolveTransferEndpoint(_ context.Context, location domain.TransferLocation) (domain.TransferEndpoint, error) {
	return domain.TransferEndpoint{URI: location.URI}, nil
}

func workspaceURL(path string) string { return (&url.URL{Scheme: "file", Path: path}).String() }

func TestWorkspaceRsyncSSHArgumentsKeepRemotePathAbsolute(t *testing.T) {
	remote := domain.TransferEndpoint{URI: "ssh://akoflow@example.test/akoflow/workspace/runs/run-1/producer"}
	local := domain.TransferEndpoint{URI: workspaceURL(t.TempDir())}
	for _, endpoints := range [][2]domain.TransferEndpoint{{remote, local}, {local, remote}} {
		args, err := rsyncArgs(endpoints[0], endpoints[1])
		if err != nil {
			t.Fatal(err)
		}
		if !containsArgument(args, "--secluded-args") {
			t.Fatalf("rsync SSH arguments lack --secluded-args: %q", args)
		}
		want := "akoflow@example.test:/akoflow/workspace/runs/run-1/producer/"
		if !containsArgument(args, want) {
			t.Fatalf("remote path was quoted or changed: %q", args)
		}
	}
}

func containsArgument(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func TestWorkspaceRsyncMergesPredecessorsBeforeSuccessorStarts(t *testing.T) {
	root := t.TempDir()
	first, second, target := filepath.Join(root, "first"), filepath.Join(root, "second"), filepath.Join(root, "target")
	for _, dir := range []string{first, second, target} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for file, content := range map[string]string{filepath.Join(first, "a.txt"): "from-first", filepath.Join(second, "b.txt"): "from-second"} {
		if err := os.WriteFile(file, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	plans := []domain.DataTransferPlan{
		{ID: "one", ExecutionRunID: "run", ProducerActivityID: "first", ConsumerActivityID: "consumer", Source: domain.TransferLocation{URI: workspaceURL(first)}, Destination: domain.TransferLocation{URI: workspaceURL(target)}},
		{ID: "two", ExecutionRunID: "run", ProducerActivityID: "second", ConsumerActivityID: "consumer", Source: domain.TransferLocation{URI: workspaceURL(second)}, Destination: domain.TransferLocation{URI: workspaceURL(target)}},
	}
	syncer := WorkspaceRsync{Resolver: workspaceEndpointResolver{}}
	runs, err := syncer.Sync(context.Background(), plans)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(target, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, run := range runs {
		if run.Status != domain.TransferCompleted || run.FilesTransferred != 1 || run.TransferredBytes == 0 {
			t.Fatalf("unexpected transfer %+v", run)
		}
	}
	runs, err = syncer.Sync(context.Background(), plans)
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range runs {
		if run.FilesTransferred != 0 || run.TransferredBytes != 0 {
			t.Fatalf("retry sent unchanged content: %+v", run)
		}
	}
}

func TestWorkspaceRsyncRejectsConflictingPredecessors(t *testing.T) {
	root := t.TempDir()
	first, second, target := filepath.Join(root, "first"), filepath.Join(root, "second"), filepath.Join(root, "target")
	for _, dir := range []string{first, second, target} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for file, content := range map[string]string{filepath.Join(first, "shared.txt"): "one", filepath.Join(second, "shared.txt"): "two"} {
		if err := os.WriteFile(file, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	plans := []domain.DataTransferPlan{
		{ID: "one", Source: domain.TransferLocation{URI: workspaceURL(first)}, Destination: domain.TransferLocation{URI: workspaceURL(target)}},
		{ID: "two", Source: domain.TransferLocation{URI: workspaceURL(second)}, Destination: domain.TransferLocation{URI: workspaceURL(target)}},
	}
	runs, err := (WorkspaceRsync{Resolver: workspaceEndpointResolver{}}).Sync(context.Background(), plans)
	if err == nil || !strings.Contains(err.Error(), "conflicting contents") {
		t.Fatalf("want conflict, got %v", err)
	}
	for _, run := range runs {
		if run.Status != domain.TransferFailed || run.Error == "" {
			t.Fatalf("missing failure: %+v", run)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "shared.txt")); !os.IsNotExist(err) {
		t.Fatalf("published conflicting file: %v", err)
	}
}

func TestWorkspaceRsyncRejectsDifferentExistingSuccessorFile(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	for _, dir := range []string{source, target} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "same.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "same.txt"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	plan := domain.DataTransferPlan{ID: "edge", Source: domain.TransferLocation{URI: workspaceURL(source)}, Destination: domain.TransferLocation{URI: workspaceURL(target)}}
	runs, err := (WorkspaceRsync{Resolver: workspaceEndpointResolver{}}).Sync(context.Background(), []domain.DataTransferPlan{plan})
	if err == nil || !strings.Contains(err.Error(), "different content") {
		t.Fatalf("want destination conflict, got %v", err)
	}
	if runs[0].Status != domain.TransferFailed {
		t.Fatalf("unexpected status %+v", runs[0])
	}
	contents, err := os.ReadFile(filepath.Join(target, "same.txt"))
	if err != nil || string(contents) != "old" {
		t.Fatalf("destination overwritten: %q %v", contents, err)
	}
}

func TestWorkspaceRsyncRejectsDestinationSymlink(t *testing.T) {
	root := t.TempDir()
	source, target, outside := filepath.Join(root, "source"), filepath.Join(root, "target"), filepath.Join(root, "outside")
	for _, dir := range []string{source, target, outside} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(source, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "file.txt"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(target, "nested")); err != nil {
		t.Fatal(err)
	}
	plan := domain.DataTransferPlan{ID: "edge", Source: domain.TransferLocation{URI: workspaceURL(source)}, Destination: domain.TransferLocation{URI: workspaceURL(target)}}
	_, err := (WorkspaceRsync{Resolver: workspaceEndpointResolver{}}).Sync(context.Background(), []domain.DataTransferPlan{plan})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("want symlink refusal, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("wrote outside destination: %v", err)
	}
}
