package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestAdapterRunsLocalActivity(t *testing.T) {
	adapter := New()
	handle, err := adapter.Start(context.Background(), domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run"},
		Activity: domain.Activity{
			ID: "activity",
			Command: domain.ActivityCommand{
				Entrypoint: "sh", Arguments: []string{"-c", "exit 0"},
			},
		},
		Resource: domain.Resource{ID: "local"}, RuntimeID: "local",
	})
	if err != nil {
		t.Fatal(err)
	}
	if handle.ID != "run:activity" {
		t.Fatalf("handle id = %q, want run:activity", handle.ID)
	}
	deadline := time.Now().Add(time.Second)
	for handle.Status == domain.HandleRunning && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		updated, err := adapter.Inspect(context.Background(), handle)
		if err != nil {
			t.Fatal(err)
		}
		handle = updated
	}
	if handle.Status != domain.HandleCompleted {
		t.Fatalf("handle=%+v", handle)
	}
}

func TestAdapterObservesGeneratedArtifacts(t *testing.T) {
	root := t.TempDir()
	adapter := New()
	handle, err := adapter.Start(context.Background(), domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run"},
		Activity: domain.Activity{ID: "activity", Command: domain.ActivityCommand{
			Entrypoint: "sh", Arguments: []string{"-c", "printf result > output.txt"}, WorkingDirectory: root,
		}},
		Resource: domain.Resource{ID: "local"}, RuntimeID: "local",
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for handle.Status == domain.HandleRunning && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		handle, err = adapter.Inspect(context.Background(), handle)
		if err != nil {
			t.Fatal(err)
		}
	}
	if handle.Artifacts == nil || len(handle.Artifacts.Files) != 1 {
		t.Fatalf("artifacts=%+v", handle.Artifacts)
	}
	artifact := handle.Artifacts.Files[0]
	if artifact.Path != "output.txt" || artifact.Change != domain.ArtifactCreated || artifact.SizeBytes != 6 {
		t.Fatalf("artifact=%+v", artifact)
	}
	if _, err := os.Stat(filepath.Join(root, "output.txt")); err != nil {
		t.Fatal(err)
	}
}
