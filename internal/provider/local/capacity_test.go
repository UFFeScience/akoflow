package local

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestCapacityUsesDockerExecutionDevice(t *testing.T) {
	capacity, err := detectCapacity(context.Background(), &executorStub{output: []byte("8 17179869184\n")})
	if err != nil || capacity.MaxCPUs != 8 || capacity.MaxMemoryBytes != 16<<30 || capacity.Source != "docker" {
		t.Fatalf("capacity=%+v err=%v", capacity, err)
	}
}

func TestBoundedMemoryHonorsContainerLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.max")
	if err := os.WriteFile(path, []byte("1073741824"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := boundedMemory(8<<30, path); got != 1<<30 {
		t.Fatalf("memory=%d", got)
	}
	if err := os.WriteFile(path, []byte("max"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := boundedMemory(8<<30, path); got != 8<<30 {
		t.Fatalf("memory=%d", got)
	}
}

func TestAllocatedLocalContainerReceivesCpuAndMemoryLimits(t *testing.T) {
	bin := t.TempDir()
	captured := filepath.Join(bin, "arguments")
	script := `#!/bin/sh
if [ "$1" = run ]; then
 printf '%s\n' "$@" > "$LOCAL_CAPACITY_ARGS_FILE"
 printf 'fake-container\n'
fi
`
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LOCAL_CAPACITY_ARGS_FILE", captured)
	t.Setenv("AKOFLOW_LOCAL_WORKSPACE_VOLUME", "")
	resource := domain.Resource{
		ID: "local", Type: domain.ResourceLocalMachine, CPUCores: 2, CPUCapacity: 2, MemoryBytes: 4 << 30,
		Metadata: map[string]any{"localAllocation": true},
	}
	_, err := New().Start(context.Background(), domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "limits"}, RuntimeID: "local", Resource: resource,
		Activity: domain.Activity{ID: "task", Command: domain.ActivityCommand{
			Image: "busybox:1.36", Entrypoint: "true", WorkingDirectory: t.TempDir(),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(captured)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for flag, want := range map[string]string{"--cpus": "1", "--memory": "2147483648", "--memory-swap": "2147483648"} {
		i := slices.Index(args, flag)
		if i < 0 || i+1 >= len(args) || args[i+1] != want {
			t.Fatalf("missing %s %s: %v", flag, want, args)
		}
	}
}
