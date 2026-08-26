// Package remote implements direct Docker execution on an SSH-connected host.
// It intentionally has no scheduler dependency.
package remote

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
)

type Factory struct{ Executor runtimecommon.CommandExecutor }

func (Factory) Driver() domain.RuntimeDriver { return domain.RuntimeDriverSSH }

func (f Factory) Build(_ domain.EnvironmentRuntime, connection domain.EnvironmentConnection) (ports.RuntimeAdapter, error) {
	if connection.Type != domain.ConnectionSSH || !directDocker(connection) {
		return nil, fmt.Errorf("SSH runtime requires a direct Docker SSH connection")
	}
	executor := f.Executor
	if executor == nil {
		executor = runtimecommon.OSCommandExecutor{}
	}
	return &Adapter{executor: runtimecommon.NewSSHCommandExecutor(executor, connection)}, nil
}

type Adapter struct{ executor runtimecommon.CommandExecutor }

func (*Adapter) Modes() []domain.ExecutionMode {
	return []domain.ExecutionMode{domain.ExecutionModeReal}
}

func (a *Adapter) Start(ctx context.Context, execution domain.ActivityExecutionContext) (domain.ActivityHandle, error) {
	if execution.Preparation != nil {
		if err := execution.Preparation.Ready(); err != nil {
			return domain.ActivityHandle{}, err
		}
	}
	activity := execution.Activity
	if strings.TrimSpace(activity.Command.Image) == "" {
		return domain.ActivityHandle{}, fmt.Errorf("activity image is required for remote Docker execution")
	}
	name := "akoflow-" + safeName(execution.Run.ID) + "-" + safeName(activity.ID)
	args := []string{"run", "--detach", "--name", name, "--label", "akoflow.run=" + execution.Run.ID}
	for key, value := range activity.Command.Environment {
		args = append(args, "--env", key+"="+value)
	}
	if activity.Resources.CPU > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(activity.Resources.CPU, 'f', -1, 64))
	}
	if activity.Resources.MemoryBytes > 0 {
		args = append(args, "--memory", strconv.FormatInt(activity.Resources.MemoryBytes, 10))
	}
	args = append(args, activity.Command.Image)
	if activity.Command.Entrypoint != "" {
		args = append(args, activity.Command.Entrypoint)
	}
	args = append(args, activity.Command.Arguments...)
	output, err := a.executor.Run(ctx, "docker", args, nil)
	if err != nil {
		return domain.ActivityHandle{}, fmt.Errorf("start remote Docker container: %w", err)
	}
	now := runtimecommon.UnixSeconds(time.Now())
	return domain.ActivityHandle{ID: runtimecommon.NewID("activity"), RunID: execution.Run.ID, ActivityID: activity.ID,
		ResourceID: execution.Resource.ID, RuntimeID: execution.RuntimeID, ExternalID: strings.TrimSpace(string(output)),
		Status: domain.HandleStarting, StartedAt: now, Metadata: map[string]any{"containerName": name, "executionTarget": "remote-docker"}}, nil
}

func (a *Adapter) Inspect(ctx context.Context, handle domain.ActivityHandle) (domain.ActivityHandle, error) {
	output, err := a.executor.Run(ctx, "docker", []string{"inspect", "--format", "{{.State.Status}}|{{.State.ExitCode}}", handle.ExternalID}, nil)
	if err != nil {
		return handle, err
	}
	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	switch parts[0] {
	case "running", "created":
		handle.Status = domain.HandleRunning
	case "exited":
		code, _ := strconv.Atoi(parts[1])
		handle.ExitCode = &code
		handle.FinishedAt = runtimecommon.UnixSeconds(time.Now())
		if code == 0 {
			handle.Status = domain.HandleCompleted
		} else {
			handle.Status = domain.HandleFailed
			handle.Failure = "remote Docker container exited with code " + strconv.Itoa(code)
		}
	default:
		handle.Status = domain.HandleFailed
		handle.Failure = "remote Docker container state: " + parts[0]
	}
	if log, logErr := a.executor.Run(ctx, "docker", []string{"logs", handle.ExternalID}, nil); logErr == nil {
		handle.Log = string(log)
	}
	return handle, nil
}

func (a *Adapter) Stop(ctx context.Context, handle domain.ActivityHandle) error {
	_, err := a.executor.Run(ctx, "docker", []string{"rm", "--force", handle.ExternalID}, nil)
	return err
}

func directDocker(c domain.EnvironmentConnection) bool {
	value, _ := c.Configuration["adapter"].(string)
	return value == "ssh-docker"
}
func intConfig(c map[string]any, key string) int {
	switch v := c[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 22
	}
}
func credentialFile(ref string) string { return strings.TrimPrefix(ref, "file:") }
func safeName(value string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, value)
}
