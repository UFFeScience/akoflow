package slurm

import (
	"context"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainconsole "github.com/UFFeScience/akoflow/internal/domain/console"
)

type consoleCaptureExecutor struct{ script string }

func (e *consoleCaptureExecutor) Run(_ context.Context, _ string, _ []string, input []byte) ([]byte, error) {
	e.script = string(input)
	return nil, nil
}

func TestConsoleSkipsSchedulerOnDirectConnection(t *testing.T) {
	executor := &consoleCaptureExecutor{}
	_, _, _, _, err := (ConsoleRunner{Executor: executor}).RunConsoleCommand(context.Background(), domain.EnvironmentConnection{Type: domain.ConnectionLocal, Configuration: map[string]any{"skipSchedulerCheck": true}}, domain.Resource{}, domainconsole.Command{Command: "hostname"})
	if err != nil || strings.Contains(executor.script, "srun") || !strings.Contains(executor.script, "exec /bin/sh -lc") {
		t.Fatalf("script=%q err=%v", executor.script, err)
	}
}

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
