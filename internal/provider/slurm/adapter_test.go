package slurm

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

func TestBatchScriptMakesSharedImageSeedAvailableWithoutCopying(t *testing.T) {
	root := t.TempDir()
	installFakeContainerRuntime(t, true, []string{
		"/akoflow-wfa-shared/input.fits",
		"/akoflow-wfa-shared/nested/catalog.tbl",
	})
	workspace := filepath.Join(root, "workspace with spaces [seed]")
	if err := os.MkdirAll(filepath.Join(workspace, "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "nested", "catalog.tbl"), []byte("materialized"), 0o600); err != nil {
		t.Fatal(err)
	}
	activity := domain.Activity{ID: "seeded", Command: domain.ActivityCommand{
		Image: "montage.sif", WorkingDirectory: workspace, Entrypoint: "sh",
		Arguments: []string{"-c", `test -L input.fits && test "$(readlink input.fits)" = /akoflow-wfa-shared/input.fits && test "$(cat nested/catalog.tbl)" = materialized && printf output > result.txt`},
	}}
	values, runErr := runBatchScript(t, root, activity)
	if runErr != nil {
		t.Fatal(runErr)
	}
	if _, err := os.Lstat(filepath.Join(workspace, "input.fits")); !os.IsNotExist(err) {
		t.Fatalf("temporary seed link remains: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(workspace, "nested", "catalog.tbl")); err != nil || string(content) != "materialized" {
		t.Fatalf("existing input changed: content=%q err=%v", content, err)
	}
	manifest := slurmArtifacts(domain.ActivityHandle{RunID: "run", ActivityID: "seeded", RuntimeID: "slurm"}, values)
	if len(manifest.Files) != 1 || manifest.Files[0].Path != "result.txt" {
		t.Fatalf("seed leaked into outputs: %+v", manifest.Files)
	}
}

func TestBatchScriptCleansSharedSeedLinksAfterFailure(t *testing.T) {
	root := t.TempDir()
	installFakeContainerRuntime(t, true, []string{"/akoflow-wfa-shared/nested/input.fits"})
	workspace := filepath.Join(root, "failed workspace")
	activity := domain.Activity{ID: "failed-seed", Command: domain.ActivityCommand{
		Image: "montage.sif", WorkingDirectory: workspace, Entrypoint: "sh",
		Arguments: []string{"-c", "test -L nested/input.fits; exit 9"},
	}}
	values, runErr := runBatchScript(t, root, activity)
	if runErr == nil || values["state"] != "failed" || values["exit_code"] != "9" {
		t.Fatalf("state=%v err=%v", values, runErr)
	}
	if links, err := filepath.Glob(filepath.Join(workspace, "**", "*.fits")); err != nil || len(links) != 0 {
		t.Fatalf("temporary links remain: %v err=%v", links, err)
	}
}

func TestBatchScriptIgnoresImageWithoutSharedSeed(t *testing.T) {
	root := t.TempDir()
	installFakeContainerRuntime(t, false, nil)
	workspace := filepath.Join(root, "ordinary")
	activity := domain.Activity{ID: "ordinary", Command: domain.ActivityCommand{
		Image: "ordinary.sif", WorkingDirectory: workspace, Entrypoint: "sh",
		Arguments: []string{"-c", "printf normal > output.txt"},
	}}
	values, runErr := runBatchScript(t, root, activity)
	if runErr != nil || values["state"] != "completed" {
		t.Fatalf("values=%v err=%v", values, runErr)
	}
	manifest := slurmArtifacts(domain.ActivityHandle{RunID: "run", ActivityID: "ordinary", RuntimeID: "slurm"}, values)
	if len(manifest.Files) != 1 || manifest.Files[0].Path != "output.txt" {
		t.Fatalf("manifest=%+v", manifest)
	}
}

func TestBatchScriptPreparesSeedBeforeSnapshotAndBindsWorkspace(t *testing.T) {
	script, err := batchScript("run", domain.Activity{ID: "ordered", Command: domain.ActivityCommand{
		Image: "image.sif", WorkingDirectory: "workspace", Entrypoint: "true",
	}}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	seed := strings.Index(script, "test -d /akoflow-wfa-shared")
	snapshot := seed + strings.Index(script[seed:], `snapshot_artifacts "$artifact_before"`)
	cleanup := strings.Index(script, `while IFS= read -r seed_link`)
	finished := strings.Index(script, `finished_at=$(date`)
	if seed < 0 || snapshot < seed || cleanup < 0 || finished < cleanup {
		t.Fatalf("invalid seed lifecycle ordering:\n%s", script)
	}
	for _, expected := range []string{
		`container_runtime=$(command -v apptainer || command -v singularity)`,
		`${container_runtime:-singularity} exec --bind "$artifact_root:$artifact_root" 'image.sif'`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("script lacks %q:\n%s", expected, script)
		}
	}
}

func installFakeContainerRuntime(t *testing.T, hasSeed bool, seedPaths []string) {
	t.Helper()
	directory := t.TempDir()
	script := `#!/bin/sh
test "$1" = exec || exit 64
shift
while [ "$1" = --bind ]; do shift 2; done
image=$1
shift
if [ "$1" = test ] && [ "$2" = -d ] && [ "$3" = /akoflow-wfa-shared ]; then
  [ "${AKOFLOW_TEST_HAS_SEED:-false}" = true ]
  exit
fi
if [ "$1" = find ] && [ "$2" = /akoflow-wfa-shared ]; then
  printf '%s\n' ${AKOFLOW_TEST_SEED_PATHS:-}
  exit
fi
exec "$@"
`
	for _, name := range []string{"singularity", "apptainer"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AKOFLOW_TEST_HAS_SEED", strconv.FormatBool(hasSeed))
	t.Setenv("AKOFLOW_TEST_SEED_PATHS", strings.Join(seedPaths, " "))
}

func runBatchScript(t *testing.T, root string, activity domain.Activity) (map[string]string, error) {
	t.Helper()
	script, err := batchScript("run", activity, "", "")
	if err != nil {
		return nil, err
	}
	command := exec.Command("sh", "-c", script)
	command.Dir = root
	command.Env = append(os.Environ(), "SLURM_JOB_ID=42")
	output, runErr := command.CombinedOutput()
	sentinels, globErr := filepath.Glob(filepath.Join(root, "akoflow-*.status"))
	if globErr != nil || len(sentinels) != 1 {
		return nil, fmt.Errorf("sentinels=%v glob=%v output=%s", sentinels, globErr, output)
	}
	payload, readErr := os.ReadFile(sentinels[0])
	if readErr != nil {
		return nil, readErr
	}
	if runErr != nil {
		runErr = fmt.Errorf("run batch script: %w: %s", runErr, output)
	}
	return sentinelValues(string(payload)), runErr
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
