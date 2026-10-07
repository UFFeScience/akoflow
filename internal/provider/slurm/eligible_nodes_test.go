package slurm

import (
	"context"
	"github.com/UFFeScience/akoflow/internal/domain"
	"strings"
	"testing"
)

type placementExecutor struct {
	script      string
	proof       bool
	proofScript string
}

func (e *placementExecutor) Run(_ context.Context, name string, _ []string, input []byte) ([]byte, error) {
	switch name {
	case "scontrol":
		return []byte("NodeName=bora001 AvailableFeatures=bora,intel,cascadelake,omnipath"), nil
	case "sinfo":
		return []byte("bora003|36|191000|bora,intel,cascadelake,omnipath|idle\nbora006|36|191000|bora,intel,cascadelake,omnipath|idle"), nil
	case "sh":
		e.proof = true
		e.proofScript = string(input)
		return nil, nil
	case "sbatch":
		e.script = string(input)
		return []byte("123"), nil
	}
	return nil, nil
}

func TestFallbackSubmissionKeepsRequestsAndGuardsBeforeCommand(t *testing.T) {
	executor := &placementExecutor{}
	adapter := NewWithConfig(executor, Config{Partition: "routage", ScriptDirectory: t.TempDir(), SubmitFromStdin: true, EligibleNodeFallback: true})
	handle, err := adapter.Start(context.Background(), domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run"}, RuntimeID: "slurm",
		Resource: domain.Resource{ID: "bora", ProviderID: "bora001", Type: domain.ResourceHPCMachine, CPUCores: 36},
		Activity: domain.Activity{ID: "activity", Resources: domain.ActivityResources{CPU: 1, MemoryBytes: 256 << 20}, Command: domain.ActivityCommand{Entrypoint: "/bin/true", WorkingDirectory: "/shared/workspace"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"#SBATCH --partition=routage", "#SBATCH --nodes=1", "#SBATCH --constraint=bora&intel&cascadelake&omnipath", "#SBATCH --cpus-per-task=1", "#SBATCH --mem=256M", "fallback workspace is not accessible", "Eligible-node fallback"} {
		if !strings.Contains(executor.script, expected) {
			t.Fatalf("missing %q", expected)
		}
	}
	if strings.Contains(executor.script, "#SBATCH --nodelist=") {
		t.Fatal("fallback must not request all eligible nodes")
	}
	if !executor.proof || handle.Metadata["plannedNode"] != "bora001" || handle.Metadata["eligibleNodes"] != "bora003,bora006" {
		t.Fatalf("missing proof or provenance: %+v", handle)
	}
	if !strings.Contains(executor.proofScript, "mkdir -p -- '/shared/workspace'") {
		t.Fatal("new root activity workspace must be initialized before shared access proof")
	}
	if strings.Index(executor.script, "fallback workspace is not accessible") > strings.LastIndex(executor.script, "/bin/true") {
		t.Fatal("workspace guard runs after activity")
	}
}

func TestEligibleNodesPreserveFamilyCapacityAndState(t *testing.T) {
	rows := "bora003|36|191000|bora,intel,cascadelake,omnipath|idle\narm01|224|255000|arm,cavium|idle\nbora004|36|191000|bora,intel,cascadelake,omnipath|drain$\nbora005|16|191000|bora,intel,cascadelake,omnipath|idle\nbora006|36|100|bora,intel,cascadelake,omnipath|idle\nbora007|36|191000|bora,intel,cascadelake,omnipath|alloc"
	got, err := selectEligibleNodes(rows, "bora,intel,cascadelake,omnipath", domain.Resource{CPUCores: 36, MemoryBytes: 191000 << 20})
	if err != nil || got != "bora003,bora007" {
		t.Fatalf("nodes=%s err=%v", got, err)
	}
}

func TestEligibleNodesFailClosed(t *testing.T) {
	_, err := selectEligibleNodes("arm01|224|255000|arm,cavium|idle", "amd,zen2,diablo", domain.Resource{})
	if err == nil || !strings.Contains(err.Error(), "no eligible") {
		t.Fatal(err)
	}
}
