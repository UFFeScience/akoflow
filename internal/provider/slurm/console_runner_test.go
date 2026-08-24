package slurm

import (
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainconsole "github.com/UFFeScience/akoflow/internal/domain/console"
)

func TestConsoleScriptRunsDirectlyOnLoginNode(t *testing.T) {
	script := consoleScript(domain.Resource{ExecutionTarget: domain.ExecutionTargetDirect}, domainconsole.Command{
		Command: "hostname && pwd", WorkingDirectory: "/scratch/a b", Environment: map[string]string{"RUN": "one two"},
	})
	for _, expected := range []string{"cd '/scratch/a b'", "export RUN='one two'", "exec /bin/sh -lc 'hostname && pwd'"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("script does not contain %q:\n%s", expected, script)
		}
	}
}

func TestConsoleScriptAllocatesSpecificSlurmNode(t *testing.T) {
	script := consoleScript(domain.Resource{Type: domain.ResourceHPCMachine, ProviderID: "bora001"}, domainconsole.Command{
		Command: "nvidia-smi", CPUCores: 2, MemoryBytes: 4 << 30, TimeoutSeconds: 120,
	})
	for _, expected := range []string{"exec srun", "--nodelist=bora001", "--cpus-per-task=2", "--mem=4096M", "--time=2"} {
		if !strings.Contains(script, expected) {
			t.Fatalf("script does not contain %q:\n%s", expected, script)
		}
	}
}
