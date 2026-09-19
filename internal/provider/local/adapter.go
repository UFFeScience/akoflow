package local

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
	"github.com/UFFeScience/akoflow/internal/provider/telemetry"
)

type processResult struct {
	exitCode   int
	err        error
	finishedAt float64
	log        string
	artifacts  *domain.ArtifactManifest
	observeErr error
}

type logBuffer struct {
	mu sync.RWMutex
	bytes.Buffer
}

func (b *logBuffer) Write(payload []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(payload)
}

func (b *logBuffer) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Buffer.String()
}

type Adapter struct {
	mu           sync.RWMutex
	results      map[string]processResult
	logs         map[string]*logBuffer
	observations map[string]runtimecommon.ArtifactSnapshot
}

func New() *Adapter {
	return &Adapter{results: make(map[string]processResult), logs: make(map[string]*logBuffer), observations: make(map[string]runtimecommon.ArtifactSnapshot)}
}
func (*Adapter) Modes() []domain.ExecutionMode {
	return []domain.ExecutionMode{domain.ExecutionModeReal, domain.ExecutionModeInteractive}
}

func (a *Adapter) Start(ctx context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	activity := execution.Activity
	if activity.Command.Entrypoint == "" {
		return domain.ActivityHandle{}, fmt.Errorf("activity entrypoint is required")
	}
	root, err := runtimecommon.PrepareArtifactRoot(activity, execution.Run.ID)
	if err != nil {
		return domain.ActivityHandle{}, fmt.Errorf("prepare artifact workspace: %w", err)
	}
	before, err := runtimecommon.SnapshotArtifacts(root)
	if err != nil {
		return domain.ActivityHandle{}, fmt.Errorf("snapshot artifact workspace: %w", err)
	}
	if image := strings.TrimSpace(activity.Command.Image); image != "" {
		return a.startContainer(ctx, execution, root, before, image)
	}
	command := exec.Command(activity.Command.Entrypoint, activity.Command.Arguments...)
	command.Dir = root
	command.Env = os.Environ()
	output := &logBuffer{}
	command.Stdout, command.Stderr = output, output
	for key, value := range activity.Command.Environment {
		command.Env = append(command.Env, key+"="+value)
	}
	if err := command.Start(); err != nil {
		return domain.ActivityHandle{}, fmt.Errorf("start local activity: %w", err)
	}
	startedAt := runtimecommon.UnixSeconds(time.Now())
	handle := domain.ActivityHandle{ID: execution.Run.ID + ":" + activity.ID, RunID: execution.Run.ID,
		ActivityID: activity.ID, ResourceID: execution.Resource.ID,
		RuntimeID: execution.RuntimeID, ExternalID: strconv.Itoa(command.Process.Pid),
		Status: domain.HandleRunning, StartedAt: startedAt,
		Endpoints: localEndpoints(activity), Metadata: map[string]any{
			domain.TimingSubmittedAt:    startedAt,
			"artifactObservationDriver": "filesystem-diff",
			"artifactObservationRoot":   root,
			"localContainerName":        "",
		}}
	a.mu.Lock()
	a.logs[handle.ID] = output
	a.observations[handle.ID] = before
	a.mu.Unlock()
	go func(id string) {
		err := command.Wait()
		exitCode := command.ProcessState.ExitCode()
		finishedAt := runtimecommon.UnixSeconds(time.Now())
		a.mu.RLock()
		before := a.observations[id]
		a.mu.RUnlock()
		after, observeErr := runtimecommon.SnapshotArtifacts(root)
		var artifacts *domain.ArtifactManifest
		if observeErr == nil {
			artifacts = runtimecommon.ArtifactManifestFor(execution.Run.ID, activity.ID, execution.RuntimeID, handle.StartedAt, finishedAt, exitCode, before, after)
		}
		a.mu.Lock()
		a.results[id] = processResult{exitCode: exitCode, err: err, finishedAt: finishedAt, log: output.String(), artifacts: artifacts, observeErr: observeErr}
		a.mu.Unlock()
	}(handle.ID)
	return handle, nil
}

func (a *Adapter) startContainer(ctx context.Context, execution domain.ActivityExecutionContext, root string, before runtimecommon.ArtifactSnapshot, image string) (domain.ActivityHandle, error) {
	activity := execution.Activity
	name := "akoflow-" + strings.NewReplacer(":", "-", "/", "-").Replace(execution.Run.ID+"-"+activity.ID)
	args := []string{"run", "--detach", "--name", name, "--label", "akoflow.run=" + execution.Run.ID}
	if volume := os.Getenv("AKOFLOW_LOCAL_WORKSPACE_VOLUME"); volume != "" {
		mountRoot := os.Getenv("AKOFLOW_LOCAL_WORKSPACE_ROOT")
		if mountRoot == "" || !strings.HasPrefix(root, filepath.Clean(mountRoot)+string(os.PathSeparator)) {
			return domain.ActivityHandle{}, fmt.Errorf("local workspace %q is outside the configured shared volume", root)
		}
		args = append(args, "--mount", "type=volume,source="+volume+",target="+mountRoot)
	} else {
		args = append(args, "--mount", "type=bind,source="+root+",target="+root)
	}
	args = append(args, "--workdir", root)
	for key, value := range activity.Command.Environment {
		args = append(args, "--env", key+"="+value)
	}
	args = append(args, image)
	if seed, _ := activity.Metadata["workspaceSeedPath"].(string); seed != "" {
		args = append(args, "sh", "-c", `cp -an "$1"/. "$2"/ && cd "$2" && shift 2 && exec "$@"`, "akoflow-seed", seed, root)
	}
	args = append(args, activity.Command.Entrypoint)
	args = append(args, activity.Command.Arguments...)
	output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return domain.ActivityHandle{}, fmt.Errorf("start local activity: %w: %s", err, strings.TrimSpace(string(output)))
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	containerID := strings.TrimSpace(lines[len(lines)-1])
	if containerID == "" {
		return domain.ActivityHandle{}, fmt.Errorf("start local activity: missing container ID")
	}
	startedAt := runtimecommon.UnixSeconds(time.Now())
	handle := domain.ActivityHandle{ID: execution.Run.ID + ":" + activity.ID, RunID: execution.Run.ID,
		ActivityID: activity.ID, ResourceID: execution.Resource.ID, RuntimeID: execution.RuntimeID,
		ExternalID: containerID, Status: domain.HandleRunning, StartedAt: startedAt,
		Endpoints: localEndpoints(activity), Metadata: map[string]any{
			domain.TimingSubmittedAt: startedAt, "artifactObservationDriver": "filesystem-diff",
			"artifactObservationRoot": root, "localContainerName": name, "executionTarget": "local-docker",
		}}
	a.mu.Lock()
	a.logs[handle.ID], a.observations[handle.ID] = &logBuffer{}, before
	a.mu.Unlock()
	telemetry.ObserveDocker(ctx, &handle, containerID, func(ctx context.Context, command string, args []string, _ []byte) ([]byte, error) {
		return exec.CommandContext(ctx, command, args...).CombinedOutput()
	})
	return handle, nil
}

func (a *Adapter) Inspect(ctx context.Context, handle domain.ActivityHandle) (domain.ActivityHandle, error) {
	if containerName, _ := handle.Metadata["localContainerName"].(string); containerName != "" {
		return a.inspectContainer(ctx, handle)
	}
	a.mu.RLock()
	result, done := a.results[handle.ID]
	a.mu.RUnlock()
	if done {
		handle.Log = result.log
		handle.FinishedAt = result.finishedAt
		handle.ExitCode = &result.exitCode
		handle.Artifacts = result.artifacts
		if result.observeErr != nil {
			if handle.Metadata == nil {
				handle.Metadata = make(map[string]any)
			}
			handle.Metadata["artifactObservationError"] = result.observeErr.Error()
		}
		if result.err != nil {
			handle.Status = domain.HandleFailed
			handle.Failure = result.err.Error()
		} else {
			handle.Status = domain.HandleCompleted
		}
		return handle, nil
	}
	a.mu.RLock()
	output := a.logs[handle.ID]
	a.mu.RUnlock()
	if output != nil {
		handle.Log = output.String()
	}
	// A short-lived process can exit after the results lookup above but before
	// its Wait goroutine persists the final result. Keep it running for this
	// polling cycle; the next inspection reads the authoritative exit status.
	// Probing the PID here is also unsafe because a PID can be reused.
	return handle, nil
}

func (a *Adapter) inspectContainer(ctx context.Context, handle domain.ActivityHandle) (domain.ActivityHandle, error) {
	output, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.Status}}|{{.State.ExitCode}}|{{.State.FinishedAt}}", handle.ExternalID).CombinedOutput()
	if err != nil {
		return handle, fmt.Errorf("inspect local container: %w: %s", err, strings.TrimSpace(string(output)))
	}
	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) < 2 {
		return handle, fmt.Errorf("inspect local container returned invalid state %q", strings.TrimSpace(string(output)))
	}
	if log, logErr := exec.CommandContext(ctx, "docker", "logs", handle.ExternalID).CombinedOutput(); logErr == nil {
		handle.Log = string(log)
	}
	if parts[0] == "running" || parts[0] == "created" {
		handle.Status = domain.HandleRunning
		telemetry.ObserveDocker(ctx, &handle, handle.ExternalID,
			func(ctx context.Context, command string, args []string, _ []byte) ([]byte, error) {
				return exec.CommandContext(ctx, command, args...).CombinedOutput()
			})
		return handle, nil
	}
	if parts[0] != "exited" {
		handle.Status = domain.HandleFailed
		handle.Failure = "local Docker container state: " + parts[0]
		return handle, nil
	}
	exitCode, _ := strconv.Atoi(parts[1])
	handle.ExitCode = &exitCode
	handle.FinishedAt = runtimecommon.UnixSeconds(time.Now())
	if len(parts) > 2 {
		if finished, parseErr := time.Parse(time.RFC3339Nano, parts[2]); parseErr == nil {
			handle.FinishedAt = runtimecommon.UnixSeconds(finished)
		}
	}
	handle.Status = domain.HandleCompleted
	if exitCode != 0 {
		handle.Status = domain.HandleFailed
		handle.Failure = "local Docker container exited with code " + strconv.Itoa(exitCode)
	}
	root, _ := handle.Metadata["artifactObservationRoot"].(string)
	a.mu.RLock()
	before, found := a.observations[handle.ID]
	a.mu.RUnlock()
	if root != "" && found {
		after, observeErr := runtimecommon.SnapshotArtifacts(root)
		if observeErr != nil {
			handle.Metadata["artifactObservationError"] = observeErr.Error()
			if exitCode == 0 {
				handle.Status = domain.HandleFailed
				handle.Failure = "observe local workspace: " + observeErr.Error()
			}
		} else {
			handle.Artifacts = runtimecommon.ArtifactManifestFor(handle.RunID, handle.ActivityID, handle.RuntimeID,
				handle.StartedAt, handle.FinishedAt, exitCode, before, after)
		}
	}
	return handle, nil
}

func (*Adapter) Stop(_ context.Context, handle domain.ActivityHandle) error {
	if name, _ := handle.Metadata["localContainerName"].(string); name != "" {
		return exec.Command("docker", "rm", "--force", name).Run()
	}
	pid, err := strconv.Atoi(handle.ExternalID)
	if err != nil {
		return fmt.Errorf("invalid local process id: %w", err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(os.Interrupt)
}

func (*Adapter) Release(ctx context.Context, handle domain.ActivityHandle) error {
	if name, _ := handle.Metadata["localContainerName"].(string); name != "" {
		output, err := exec.CommandContext(ctx, "docker", "rm", "--force", handle.ExternalID).CombinedOutput()
		if err != nil && !strings.Contains(string(output), "No such container") {
			return fmt.Errorf("remove local container: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func localEndpoints(activity domain.Activity) []string {
	if activity.Service == nil {
		return nil
	}
	result := make([]string, 0, len(activity.Service.Ports))
	for _, port := range activity.Service.Ports {
		result = append(result, fmt.Sprintf("tcp://127.0.0.1:%d", port))
	}
	return result
}
