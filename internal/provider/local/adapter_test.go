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

func TestAdapterFailureValidationEndpointsAndStop(t *testing.T) {
	adapter := New()
	if modes := adapter.Modes(); len(modes) != 2 || modes[1] != domain.ExecutionModeInteractive {
		t.Fatalf("modes=%#v", modes)
	}
	if _, err := adapter.Start(context.Background(), domain.ActivityExecutionContext{Activity: domain.Activity{ID: "missing"}}); err == nil {
		t.Fatal("expected entrypoint error")
	}
	if _, err := adapter.Start(context.Background(), domain.ActivityExecutionContext{Run: domain.ExecutionRun{ID: "run"}, Activity: domain.Activity{ID: "bad", Command: domain.ActivityCommand{Entrypoint: "/missing/program"}}}); err == nil {
		t.Fatal("expected start error")
	}
	handle, err := adapter.Start(context.Background(), domain.ActivityExecutionContext{Run: domain.ExecutionRun{ID: "run"}, Activity: domain.Activity{ID: "failed", Command: domain.ActivityCommand{Entrypoint: "sh", Arguments: []string{"-c", "echo failure; exit 7"}, Environment: map[string]string{"VALUE": "set"}}, Service: &domain.ServiceSpec{Ports: []int{8080, 9090}}}, Resource: domain.Resource{ID: "local"}, RuntimeID: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if len(handle.Endpoints) != 2 || handle.Endpoints[0] != "tcp://127.0.0.1:8080" {
		t.Fatalf("endpoints=%#v", handle.Endpoints)
	}
	deadline := time.Now().Add(time.Second)
	for handle.Status == domain.HandleRunning && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		handle, err = adapter.Inspect(context.Background(), handle)
		if err != nil {
			t.Fatal(err)
		}
	}
	if handle.Status != domain.HandleFailed || handle.ExitCode == nil || *handle.ExitCode != 7 || handle.Log != "failure\n" {
		t.Fatalf("handle=%#v", handle)
	}
	if err = adapter.Stop(context.Background(), domain.ActivityHandle{ExternalID: "invalid"}); err == nil {
		t.Fatal("expected invalid PID error")
	}
	if localEndpoints(domain.Activity{}) != nil {
		t.Fatal("non-service endpoints must be nil")
	}
}
