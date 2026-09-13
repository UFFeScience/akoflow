package remote

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	provider "github.com/UFFeScience/akoflow/internal/provider"
)

type commandExecutorStub struct {
	calls     []commandCall
	responses [][]byte
	errors    []error
}
type commandCall struct {
	name string
	args []string
}

func (s *commandExecutorStub) Run(_ context.Context, name string, args []string, _ []byte) ([]byte, error) {
	s.calls = append(s.calls, commandCall{name: name, args: append([]string(nil), args...)})
	index := len(s.calls) - 1
	var response []byte
	var err error
	if index < len(s.responses) {
		response = s.responses[index]
	}
	if index < len(s.errors) {
		err = s.errors[index]
	}
	return response, err
}

func executionFixture() domain.ActivityExecutionContext {
	return domain.ActivityExecutionContext{
		Run: domain.ExecutionRun{ID: "run:one"}, RuntimeID: "runtime",
		Activity: domain.Activity{
			ID: "activity/one",
			Command: domain.ActivityCommand{
				Image: "ubuntu:latest", Entrypoint: "sh", Arguments: []string{"-c", "echo ok"},
				Environment: map[string]string{"A": "B"},
			},
			Resources: domain.ActivityResources{CPU: 2.5, MemoryBytes: 1024},
		},
		Resource: domain.Resource{ID: "machine"},
	}
}

func TestFactoryValidatesDirectDockerConnection(t *testing.T) {
	factory := Factory{}
	if factory.Driver() != domain.RuntimeDriverSSH {
		t.Fatalf("driver = %s", factory.Driver())
	}
	for _, connection := range []domain.EnvironmentConnection{{Type: domain.ConnectionLocal}, {Type: domain.ConnectionSSH}} {
		if _, err := factory.Build(domain.EnvironmentRuntime{}, connection); err == nil {
			t.Fatalf("expected rejection for %#v", connection)
		}
	}
	adapter, err := factory.Build(domain.EnvironmentRuntime{}, domain.EnvironmentConnection{Type: domain.ConnectionSSH, Configuration: map[string]any{"adapter": "ssh-docker"}})
	if err != nil || adapter == nil {
		t.Fatalf("Build() = %#v, %v", adapter, err)
	}
}

func TestAdapterStartsRemoteContainerWithResourceLimits(t *testing.T) {
	executor := &commandExecutorStub{responses: [][]byte{nil, []byte("[]"), []byte("Unable to find image locally\nPull complete\ncontainer-id\n")}}
	adapter := &Adapter{executor: executor}
	handle, err := adapter.Start(context.Background(), executionFixture())
	if err != nil {
		t.Fatal(err)
	}
	if handle.ExternalID != "container-id" || handle.Status != domain.HandleStarting || handle.Metadata["containerName"] != "akoflow-run-one-activity-one" {
		t.Fatalf("handle = %#v", handle)
	}
	if executor.calls[0].name != "mkdir" || executor.calls[1].name != "python3" {
		t.Fatalf("workspace preparation calls = %#v", executor.calls[:2])
	}
	call := executor.calls[2]
	joined := strings.Join(call.args, " ")
	for _, expected := range []string{"run --detach", "--label akoflow.run=run:one", "--env A=B", "--cpus 2.5", "--memory 1024", "ubuntu:latest sh -c echo ok"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("docker args %q lack %q", joined, expected)
		}
	}
	if modes := adapter.Modes(); len(modes) != 1 || modes[0] != domain.ExecutionModeReal {
		t.Fatalf("modes = %#v", modes)
	}
}

func TestAdapterRejectsUnreadyPreparationAndMissingImage(t *testing.T) {
	adapter := &Adapter{executor: &commandExecutorStub{}}
	execution := executionFixture()
	execution.Preparation = &domain.PreparationGate{Executable: &domain.ArtifactMaterialization{Status: domain.MaterializationFailed}}
	if _, err := adapter.Start(context.Background(), execution); err == nil {
		t.Fatal("expected preparation error")
	}
	execution.Preparation, execution.Activity.Command.Image = nil, " "
	if _, err := adapter.Start(context.Background(), execution); err == nil {
		t.Fatal("expected image error")
	}
	execution.Activity.Command.Image = "image"
	adapter.executor = &commandExecutorStub{responses: [][]byte{nil, []byte("[]")}, errors: []error{nil, nil, fmt.Errorf("docker unavailable")}}
	if _, err := adapter.Start(context.Background(), execution); err == nil || !strings.Contains(err.Error(), "start remote Docker") {
		t.Fatalf("start error = %v", err)
	}
}

func TestAdapterInspectsRunningCompletedFailedAndUnknownContainers(t *testing.T) {
	tests := []struct {
		state   string
		want    domain.ActivityHandleStatus
		exit    *int
		failure string
	}{
		{state: "running|0\n", want: domain.HandleRunning},
		{state: "created|0\n", want: domain.HandleRunning},
		{state: "exited|0\n", want: domain.HandleCompleted, exit: intPointer(0)},
		{state: "exited|7\n", want: domain.HandleFailed, exit: intPointer(7), failure: "exited with code 7"},
		{state: "dead|0\n", want: domain.HandleFailed, failure: "state: dead"},
	}
	for _, test := range tests {
		t.Run(strings.ReplaceAll(strings.TrimSpace(test.state), "|", "-"), func(t *testing.T) {
			executor := &commandExecutorStub{responses: [][]byte{[]byte(test.state), []byte("container log")}}
			adapter := &Adapter{executor: executor}
			handle, err := adapter.Inspect(context.Background(), domain.ActivityHandle{ExternalID: "container"})
			if err != nil || handle.Status != test.want || handle.Log != "container log" {
				t.Fatalf("Inspect() = %#v, %v", handle, err)
			}
			if test.exit != nil && (handle.ExitCode == nil || *handle.ExitCode != *test.exit) {
				t.Fatalf("exit = %#v", handle.ExitCode)
			}
			if test.failure != "" && !strings.Contains(handle.Failure, test.failure) {
				t.Fatalf("failure = %q", handle.Failure)
			}
		})
	}
}

func TestAdapterCatalogsOnlyRemoteWorkspaceChanges(t *testing.T) {
	before := `[{"path":"input.txt","size":3,"checksum":"old","modified":1},{"path":"gone.txt","size":2,"checksum":"gone","modified":1}]`
	after := `[{"path":"input.txt","size":4,"checksum":"new","modified":2},{"path":"result.txt","size":5,"checksum":"result","modified":3}]`
	executor := &commandExecutorStub{responses: [][]byte{
		[]byte("exited|0\n"), []byte(after), []byte("done\n"),
	}}
	adapter := &Adapter{executor: executor}
	handle, err := adapter.Inspect(context.Background(), domain.ActivityHandle{
		RunID: "run", ActivityID: "activity", RuntimeID: "cloud", ExternalID: "container",
		StartedAt: 10, Metadata: map[string]any{
			"artifactObservationRoot":   "/akoflow/workspace/run/activity",
			"artifactObservationBefore": before,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if handle.Status != domain.HandleCompleted || handle.Artifacts == nil {
		t.Fatalf("handle = %#v", handle)
	}
	manifest := handle.Artifacts
	if manifest.Summary.InitialFiles != 2 || manifest.Summary.FinalFiles != 2 ||
		manifest.Summary.CreatedFiles != 1 || manifest.Summary.ModifiedFiles != 1 ||
		manifest.Summary.DeletedFiles != 1 || manifest.Summary.OutputBytes != 9 {
		t.Fatalf("summary = %#v", manifest.Summary)
	}
	if len(manifest.Files) != 3 || manifest.Files[0].Path != "gone.txt" ||
		manifest.Files[1].Path != "input.txt" || manifest.Files[2].Path != "result.txt" {
		t.Fatalf("files = %#v", manifest.Files)
	}
}

func TestAdapterInspectAndStopPropagateExecutorErrors(t *testing.T) {
	executor := &commandExecutorStub{errors: []error{fmt.Errorf("inspect failed")}}
	adapter := &Adapter{executor: executor}
	if _, err := adapter.Inspect(context.Background(), domain.ActivityHandle{ExternalID: "id"}); err == nil {
		t.Fatal("expected inspect error")
	}
	executor = &commandExecutorStub{errors: []error{fmt.Errorf("remove failed")}}
	adapter.executor = executor
	if err := adapter.Stop(context.Background(), domain.ActivityHandle{ExternalID: "id"}); err == nil {
		t.Fatal("expected stop error")
	}
	if executor.calls[0].name != "docker" || strings.Join(executor.calls[0].args, " ") != "rm --force id" {
		t.Fatalf("stop call = %#v", executor.calls[0])
	}
}

func TestRemoteHelpers(t *testing.T) {
	if directDocker(domain.EnvironmentConnection{}) || !directDocker(domain.EnvironmentConnection{Configuration: map[string]any{"adapter": "ssh-docker"}}) {
		t.Fatal("directDocker mismatch")
	}
	if intConfig(map[string]any{"port": 2200.0}, "port") != 2200 || intConfig(map[string]any{"port": 2222}, "port") != 2222 || intConfig(nil, "port") != 22 {
		t.Fatal("intConfig mismatch")
	}
	if credentialFile("file:/tmp/key") != "/tmp/key" || safeName("run/a:b") != "run-a-b" {
		t.Fatal("helper mismatch")
	}
	var _ provider.CommandExecutor = &commandExecutorStub{}
}

func intPointer(value int) *int { return &value }
