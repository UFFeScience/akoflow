package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestArtifactSnapshotsProduceCreatedModifiedAndDeletedManifest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "unchanged.txt"), "same")
	writeFile(t, filepath.Join(root, "modified.txt"), "before")
	writeFile(t, filepath.Join(root, "deleted.txt"), "deleted")
	before, err := SnapshotArtifacts(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "modified.txt"), "after content")
	if err = os.Remove(filepath.Join(root, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "nested", "created.txt"), "created")
	if err = os.Symlink(filepath.Join(root, "unchanged.txt"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	after, err := SnapshotArtifacts(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest := ArtifactManifestFor("run", "activity", "local", 10, 14.5, 0, before, after)
	if manifest.Summary.InitialFiles != 3 || manifest.Summary.FinalFiles != 3 || manifest.Summary.CreatedFiles != 1 || manifest.Summary.ModifiedFiles != 1 || manifest.Summary.DeletedFiles != 1 {
		t.Fatalf("summary = %#v", manifest.Summary)
	}
	if len(manifest.Files) != 3 || manifest.Phases[0].Status != "completed" || manifest.Phases[0].DurationSeconds != 4.5 {
		t.Fatalf("manifest = %#v", manifest)
	}
	changes := map[string]domain.ArtifactChange{}
	for _, file := range manifest.Files {
		changes[file.Path] = file.Change
		if !strings.HasPrefix(file.Checksum, "sha256:") {
			t.Fatalf("checksum = %q", file.Checksum)
		}
	}
	if changes["deleted.txt"] != domain.ArtifactDeleted || changes["modified.txt"] != domain.ArtifactModified || changes["nested/created.txt"] != domain.ArtifactCreated {
		t.Fatalf("changes = %#v", changes)
	}
}

func TestPrepareArtifactRootUsesMetadataWorkingDirectoryOrTemporaryDirectory(t *testing.T) {
	metadataRoot := filepath.Join(t.TempDir(), "metadata")
	root, err := PrepareArtifactRoot(domain.Activity{Metadata: map[string]any{"artifactObservationRoot": metadataRoot}}, "run")
	if err != nil || root != metadataRoot {
		t.Fatalf("metadata root = %q, %v", root, err)
	}
	workingRoot := filepath.Join(t.TempDir(), "working")
	root, err = PrepareArtifactRoot(domain.Activity{Command: domain.ActivityCommand{WorkingDirectory: workingRoot}}, "run")
	if err != nil || root != workingRoot {
		t.Fatalf("working root = %q, %v", root, err)
	}
	root, err = PrepareArtifactRoot(domain.Activity{ID: "activity/id"}, "run:id")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	if !strings.Contains(filepath.Base(root), "akoflow-run-id-activity-id-") {
		t.Fatalf("temporary root = %q", root)
	}
}

func TestArtifactHelpersAndCommandUtilities(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	writeFile(t, file, "payload")
	checksum, err := fileChecksum(file)
	if err != nil || len(checksum) != 64 {
		t.Fatalf("checksum = %q, %v", checksum, err)
	}
	if _, err = fileChecksum(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected checksum error")
	}
	if artifactPhase(0) != "completed" || artifactPhase(1) != "failed" || maxDuration(-1) != 0 || maxDuration(2) != 2 || safePathToken("a/b:c") != "a-b-c" {
		t.Fatal("artifact helper mismatch")
	}
	output, err := (OSCommandExecutor{}).Run(context.Background(), "sh", []string{"-c", "printf hello"}, nil)
	if err != nil || string(output) != "hello" {
		t.Fatalf("command = %q, %v", output, err)
	}
	if _, err = (OSCommandExecutor{}).Run(context.Background(), "sh", []string{"-c", "echo failure; exit 2"}, nil); err == nil || !strings.Contains(err.Error(), "failure") {
		t.Fatalf("command error = %v", err)
	}
	if id := NewID("test"); !strings.HasPrefix(id, "test-") {
		t.Fatalf("id = %q", id)
	}
	now := time.Now()
	if UnixSeconds(now) <= 0 {
		t.Fatal("invalid unix seconds")
	}
}

func TestFailedArtifactManifestClampsNegativeDuration(t *testing.T) {
	manifest := ArtifactManifestFor("run", "activity", "remote", 20, 10, 1, ArtifactSnapshot{}, ArtifactSnapshot{})
	if manifest.Phases[0].Status != "failed" || manifest.Phases[0].DurationSeconds != 0 {
		t.Fatalf("phase = %#v", manifest.Phases[0])
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
