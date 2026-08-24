package local

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
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

func (a *Adapter) Start(_ context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
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

func (a *Adapter) Inspect(_ context.Context, handle domain.ActivityHandle) (domain.ActivityHandle, error) {
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

func (*Adapter) Stop(_ context.Context, handle domain.ActivityHandle) error {
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
