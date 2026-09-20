package slurm

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type executorFake struct {
	output, input []byte
	name          string
	args          []string
	calls         int
}

type failingStatusExecutor struct{}

func (failingStatusExecutor) Run(_ context.Context, name string, _ []string, _ []byte) ([]byte, error) {
	return nil, fmt.Errorf("%s unavailable", name)
}

func (f *executorFake) Run(_ context.Context, name string, args []string, input []byte) ([]byte, error) {
	f.calls++
	f.name = name
	f.args = args
	f.input = input
	return f.output, nil
}

func TestAdapterExecutesDirectlyOnLoginResource(t *testing.T) {
	adapter := NewWithConfig(&executorFake{}, Config{Partition: "cpu", ScriptDirectory: t.TempDir()})
	execution := domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run"},
		Activity: domain.Activity{ID: "lightweight", Command: domain.ActivityCommand{
			Entrypoint: "/bin/sh", Arguments: []string{"-c", "exit 0"},
		}},
		Resource: domain.Resource{ID: "login", ExecutionTarget: domain.ExecutionTargetDirect}, RuntimeID: "slurm",
	}
	handle, err := adapter.Start(context.Background(), execution)
	if err != nil {
		t.Fatal(err)
	}
	if handle.Metadata["slurmSubmission"] != "login-node" {
		t.Fatalf("metadata=%+v", handle.Metadata)
	}
	scriptPath, _ := handle.Metadata["scriptPath"].(string)
	script, err := os.ReadFile(scriptPath)
	if err != nil || !strings.Contains(string(script), "'/bin/sh' '-c' 'exit 0'") {
		t.Fatalf("script=%s err=%v", script, err)
	}
	for attempt := 0; attempt < 50; attempt++ {
		handle, err = adapter.Inspect(context.Background(), handle)
		if err != nil {
			t.Fatal(err)
		}
		if handle.Status == domain.HandleCompleted {
			executor := adapter.executor.(*executorFake)
			if executor.name != "sh" || len(executor.args) != 1 || executor.args[0] != "-s" || !strings.Contains(string(executor.input), "'/bin/sh' '-c' 'exit 0'") {
				t.Fatalf("direct execution must use the configured executor: name=%q args=%v input=%q", executor.name, executor.args, executor.input)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("direct activity did not complete: %+v", handle)
}

func TestSlurmRejectsUnmaterializedOCIInsteadOfAddingDockerURI(t *testing.T) {
	_, err := slurmExecutable(domain.ActivityCommand{Image: "alpine:3.20"})
	if err == nil || strings.Contains(err.Error(), "docker://") {
		t.Fatalf("err=%v", err)
	}
}

func TestSlurmTreatsEmptyExecutableFormAsNativeCommand(t *testing.T) {
	image, err := slurmExecutable(domain.ActivityCommand{
		Executable: &domain.ExecutableReference{}, Entrypoint: "hostname",
	})
	if err != nil || image != "" {
		t.Fatalf("image=%q err=%v", image, err)
	}
}

func TestSlurmAccountingOutageDoesNotCreateFalseJobFailure(t *testing.T) {
	handle := domain.ActivityHandle{ExternalID: "42", Status: domain.HandleRunning}
	observed, err := New(failingStatusExecutor{}, "").Inspect(context.Background(), handle)
	if err != nil || observed.Status != domain.HandleRunning || observed.Failure != "" {
		t.Fatalf("handle=%+v err=%v", observed, err)
	}
	if !strings.Contains(fmt.Sprint(observed.Metadata["statusQueryWarning"]), "inspection unavailable") {
		t.Fatalf("missing accounting warning: %+v", observed.Metadata)
	}
}

func TestSlurmUsesOnlyMaterializedOrSharedSIF(t *testing.T) {
	image, err := slurmExecutable(domain.ActivityCommand{Executable: &domain.ExecutableReference{
		Source: domain.ExecutableSource{Type: domain.ExecutableSourceRemoteFile, Path: "/apps/tool.sif", ResourceRef: "plafrim"}, Delivery: domain.ExecutableDelivery{Strategy: domain.DeliveryUseInPlace},
	}})
	if err != nil || image != "/apps/tool.sif" {
		t.Fatalf("image=%q err=%v", image, err)
	}
	image, err = slurmExecutable(domain.ActivityCommand{Executable: &domain.ExecutableReference{
		Source: domain.ExecutableSource{Type: domain.ExecutableSourceOCI, Reference: "docker://alpine:3.20"}, Delivery: domain.ExecutableDelivery{Strategy: domain.DeliveryDestinationPull},
	}})
	if err == nil || image != "" {
		t.Fatalf("image=%q err=%v", image, err)
	}
}

func TestSlurmBlocksUncommittedWorkspace(t *testing.T) {
	adapter := NewWithConfig(&executorFake{}, Config{ScriptDirectory: t.TempDir()})
	_, err := adapter.Start(context.Background(), domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run"}, RuntimeID: "slurm", Resource: domain.Resource{ID: "node"},
		Activity:    domain.Activity{ID: "task", Command: domain.ActivityCommand{Entrypoint: "echo"}},
		Preparation: &domain.PreparationGate{Workspace: &domain.WorkspaceMaterialization{Status: domain.MaterializationTransferring}},
	})
	if err == nil || !strings.Contains(err.Error(), "workspace materialization") {
		t.Fatalf("err=%v", err)
	}
}

func TestAdapterSubmitsSafeBatchScript(t *testing.T) {
	executor := &executorFake{output: []byte("123;cluster\n")}
	adapter := NewWithConfig(executor, Config{Partition: "cpu", ScriptDirectory: t.TempDir()})
	activity := domain.Activity{ID: "analysis", Command: domain.ActivityCommand{Image: "image.sif", Entrypoint: "python", Arguments: []string{"a'b"}}, Resources: domain.ActivityResources{CPU: 2, MemoryBytes: 1048576}}
	handle, err := adapter.Start(context.Background(), domain.ActivityExecutionContext{Run: domain.ExecutionRun{ID: "run"}, Activity: activity, Resource: domain.Resource{ID: "node"}, RuntimeID: "slurm"})
	if err != nil {
		t.Fatal(err)
	}
	if handle.ExternalID != "123" || executor.name != "sbatch" || len(executor.args) != 2 {
		t.Fatalf("handle=%+v command=%s args=%v", handle, executor.name, executor.args)
	}
	script, err := os.ReadFile(executor.args[1])
	if err != nil || !strings.Contains(string(script), `'a'"'"'b'`) || strings.Index(string(script), "#SBATCH") > strings.Index(string(script), "set -eu") {
		t.Fatalf("script=%s err=%v", script, err)
	}
	hasTrap := strings.Contains(string(script), "trap finish EXIT")
	hasResilientTrap := strings.Contains(string(script), "finish() { code=$?; set +e;")
	hasRunningState := strings.Contains(string(script), "state=running")
	hasAbsoluteArtifactRoot := strings.Contains(string(script), `artifact_root=$(cd "$artifact_root" && pwd -P)`)
	hasLogPath := strings.Contains(string(script), "akoflow-run-analysis-%j.log")
	hasSentinelPath := handle.Metadata["sentinelPath"] == "akoflow-run-analysis-123.status"
	if !hasTrap || !hasResilientTrap || !hasRunningState || !hasAbsoluteArtifactRoot || !hasLogPath || !hasSentinelPath {
		t.Fatalf("missing execution sentinel: script=%s metadata=%+v", script, handle.Metadata)
	}
}

func TestBatchScriptObservesFilesAfterChangingIntoRelativeWorkspace(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join("akoflow-workspaces", "run", "activity")
	script, err := batchScript("run", domain.Activity{ID: "activity", Command: domain.ActivityCommand{
		Entrypoint: "sh", Arguments: []string{"-c", "printf output > result.txt"}, WorkingDirectory: workspace,
	}}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("sh", "-c", script)
	command.Dir = root
	command.Env = append(os.Environ(), "SLURM_JOB_ID=42")
	if output, runErr := command.CombinedOutput(); runErr != nil {
		t.Fatalf("run batch script: %v: %s", runErr, output)
	}
	sentinels, err := filepath.Glob(filepath.Join(root, "akoflow-*.status"))
	if err != nil || len(sentinels) != 1 {
		entries, _ := os.ReadDir(root)
		t.Fatalf("sentinels=%v entries=%v err=%v", sentinels, entries, err)
	}
	sentinel, err := os.ReadFile(sentinels[0])
	if err != nil {
		t.Fatal(err)
	}
	values := sentinelValues(string(sentinel))
	manifest := slurmArtifacts(domain.ActivityHandle{RunID: "run", ActivityID: "activity", RuntimeID: "slurm"}, values)
	if len(manifest.Files) != 1 || manifest.Files[0].Path != "result.txt" || manifest.Files[0].SizeBytes != 6 {
		t.Fatalf("manifest=%+v sentinel=%s", manifest, sentinel)
	}
}

func TestAdapterParsesJobIDAfterSSHWarning(t *testing.T) {
	adapter := NewWithConfig(&executorFake{output: []byte("warning from ssh\n4847261;cluster\n")}, Config{ScriptDirectory: t.TempDir()})
	handle, err := adapter.Start(context.Background(), domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run"}, RuntimeID: "slurm", Resource: domain.Resource{ID: "node"},
		Activity: domain.Activity{ID: "task", Command: domain.ActivityCommand{Entrypoint: "echo", Arguments: []string{"ok"}}},
	})
	if err != nil || handle.ExternalID != "4847261" {
		t.Fatalf("handle=%+v err=%v", handle, err)
	}
}

func TestAdapterMapsSlurmStatus(t *testing.T) {
	executor := &executorFake{output: []byte("COMPLETED|0:0\n")}
	handle, err := New(executor, "").Inspect(context.Background(), domain.ActivityHandle{ExternalID: "1"})
	if err != nil || handle.Status != domain.HandleCompleted || handle.ExitCode == nil || *handle.ExitCode != 0 {
		t.Fatalf("handle=%+v err=%v", handle, err)
	}
}

func TestAdapterInspectsSlurmWithSingleCommand(t *testing.T) {
	encode := func(value string) string {
		return base64.StdEncoding.EncodeToString([]byte(value))
	}
	executor := &executorFake{output: []byte(
		"LOG=" + encode("done\n") + "\n" +
			"METRICS=\n" +
			"SENTINEL=" + encode("state=completed\nexit_code=0\nstarted_at=100\nfinished_at=104\n") + "\n" +
			"SACCT=\nSQUEUE=\nSCONTROL=\n",
	)}
	handle, err := New(executor, "").Inspect(context.Background(), domain.ActivityHandle{
		ExternalID: "1",
		Metadata: map[string]any{
			"logPath": "job.log", "metricsPath": "metrics.tsv", "sentinelPath": "job.status",
		},
	})
	if err != nil || executor.calls != 1 || executor.name != "sh" || handle.Status != domain.HandleCompleted || handle.FinishedAt != 104 || handle.Log != "done\n" {
		t.Fatalf("handle=%+v calls=%d command=%s err=%v", handle, executor.calls, executor.name, err)
	}
}

func TestSentinelAndScontrolStatusParsing(t *testing.T) {
	values := sentinelValues("state=failed\nexit_code=17\nartifact_root=/scratch/run\nartifact=b3V0cHV0LnR4dA==|42|abc123\n")
	if values["state"] != "failed" || values["exit_code"] != "17" {
		t.Fatalf("sentinel values=%+v", values)
	}
	manifest := slurmArtifacts(domain.ActivityHandle{RunID: "run", ActivityID: "activity", RuntimeID: "slurm"}, values)
	if len(manifest.Files) != 1 || manifest.Files[0].Path != "output.txt" || manifest.Files[0].SizeBytes != 42 {
		t.Fatalf("manifest=%+v", manifest)
	}
	if state := slurmControlState("JobId=123 JobState=RUNNING Reason=None"); state != "RUNNING" {
		t.Fatalf("scontrol state=%q", state)
	}
}

func TestSentinelRecordsContainerStartupTiming(t *testing.T) {
	executor := &executorFake{output: []byte("state=completed\nexit_code=0\nstarted_at=100\nallocated_node=bora017\ncontainer_started_at=103.25\n")}
	handle, found := New(executor, "").sentinelStatus(context.Background(), domain.ActivityHandle{
		Metadata: map[string]any{"sentinelPath": "status"},
	})
	if !found || handle.StartedAt != 100 || handle.Metadata[domain.TimingContainerStartedAt] != 103.25 || handle.Metadata["allocatedNode"] != "bora017" {
		t.Fatalf("handle=%+v found=%t", handle, found)
	}
}

func TestConnectionFactorySubmitsToConfiguredSSHLoginNode(t *testing.T) {
	executor := &executorFake{output: []byte("123;cluster\n")}
	adapter, err := (ConnectionFactory{Executor: executor, DefaultScriptDirectory: t.TempDir()}).Build(
		domain.EnvironmentRuntime{ID: "hpc", Driver: domain.RuntimeDriverSlurm,
			Configuration: map[string]any{"connectionId": "hpc-ssh", "partition": "cpu"}},
		domain.EnvironmentConnection{ID: "hpc-ssh", Type: domain.ConnectionSSH,
			Endpoint: "login.example", Username: "wes", Configuration: map[string]any{"port": float64(2222)}},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Start(context.Background(), domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run"}, RuntimeID: "hpc", Resource: domain.Resource{ID: "node"},
		Activity: domain.Activity{ID: "task", Command: domain.ActivityCommand{Entrypoint: "echo", Arguments: []string{"ok"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if executor.name != "ssh" || !strings.Contains(strings.Join(executor.args, " "), "wes@login.example") ||
		!strings.Contains(string(executor.input), "#SBATCH --partition=cpu") {
		t.Fatalf("name=%s args=%v input=%s", executor.name, executor.args, executor.input)
	}
}

func TestConnectionProberChecksSlurmThroughSSH(t *testing.T) {
	executor := &executorFake{output: []byte("cpu*\ngpu\n")}
	health := NewConnectionProber(executor).Probe(context.Background(), domain.EnvironmentConnection{
		Type: domain.ConnectionSSH, Endpoint: "login.example", Username: "wes",
	})
	if !health.Healthy || executor.name != "ssh" || !strings.Contains(health.Message, "cpu") {
		t.Fatalf("health=%+v command=%s", health, executor.name)
	}
}
