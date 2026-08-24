package slurm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/creack/pty"
)

var safeSchedulerTarget = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// TerminalRunner creates an actual interactive shell. SSH resources receive a
// forced TTY and scheduler resources are allocated through srun --pty.
type TerminalRunner struct{}

var _ ports.InteractiveConsoleRunner = TerminalRunner{}

func (TerminalRunner) StartInteractive(_ context.Context, connection domain.EnvironmentConnection, resource domain.Resource) (ports.InteractiveTerminal, error) {
	command, cleanup, err := interactiveCommandForJob(connection, resource, fmt.Sprintf("akoflow-console-%d", time.Now().UnixNano()))
	if err != nil {
		return nil, err
	}
	terminal, err := pty.Start(command)
	if err != nil {
		return nil, fmt.Errorf("start interactive terminal: %w", err)
	}
	return &terminalHandle{file: terminal, command: command, cleanup: cleanup}, nil
}

func interactiveCommand(connection domain.EnvironmentConnection, resource domain.Resource) (*exec.Cmd, error) {
	command, _, err := interactiveCommandForJob(connection, resource, "")
	return command, err
}

func interactiveCommandForJob(connection domain.EnvironmentConnection, resource domain.Resource, jobName string) (*exec.Cmd, func(), error) {
	if connection.Type == domain.ConnectionLocal || connection.Type == domain.ConnectionAgent {
		return exec.Command("/bin/sh", "-l"), nil, nil
	}
	if connection.Type == domain.ConnectionKubernetes {
		container, _ := resource.Metadata["interactiveDockerContainer"].(string)
		if container == "" {
			return nil, nil, fmt.Errorf("interactive Kubernetes terminals are currently available only for discovered Kind control-plane resources")
		}
		return exec.Command("docker", "exec", "-it", container, "/bin/sh", "-l"), nil, nil
	}
	if connection.Type != domain.ConnectionSSH {
		return nil, nil, fmt.Errorf("interactive terminals are not supported for connection type %q", connection.Type)
	}
	args := []string{"-tt", "-o", "BatchMode=yes", "-o", "ConnectionAttempts=1", "-o", "ConnectTimeout=10", "-o", "CheckHostIP=no"}
	if port := configInt(connection.Configuration, "port"); port > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", port))
	}
	if identityFile := credentialFile(connection.CredentialRef); identityFile != "" {
		args = append(args, "-i", identityFile)
	}
	if proxy := configString(connection.Configuration, "proxyCommand"); proxy != "" {
		args = append(args, "-o", "ProxyCommand="+proxy)
	}
	if alias := configString(connection.Configuration, "hostKeyAlias"); alias != "" {
		args = append(args, "-o", "HostKeyAlias="+alias)
	}
	if hosts := configString(connection.Configuration, "knownHostsFile"); hosts != "" {
		args = append(args, "-o", "UserKnownHostsFile="+hosts, "-o", "StrictHostKeyChecking=yes")
	}
	if configBool(connection.Configuration, "forwardAgent", false) {
		args = append(args, "-A")
	}
	target := strings.TrimSpace(connection.Endpoint)
	if username := strings.TrimSpace(connection.Username); username != "" {
		target = username + "@" + target
	}
	if target == "" {
		return nil, nil, fmt.Errorf("SSH endpoint is required for an interactive terminal")
	}
	args = append(args, target)
	if resource.ExecutionTarget == domain.ExecutionTargetDirect {
		args = append(args, "/bin/sh", "-l")
		return exec.Command("ssh", args...), nil, nil
	}
	if !safeSchedulerTarget.MatchString(resource.ProviderID) {
		return nil, nil, fmt.Errorf("invalid scheduler target %q", resource.ProviderID)
	}
	jobArgs := []string{"srun"}
	if jobName != "" {
		jobArgs = append(jobArgs, "--job-name="+jobName)
	}
	if resource.Type == domain.ResourceHPCPartition {
		jobArgs = append(jobArgs, "--partition="+resource.ProviderID, "--pty", "/bin/bash", "-l")
	} else if resource.Type == domain.ResourceHPCMachine {
		jobArgs = append(jobArgs, "--nodelist="+resource.ProviderID, "--pty", "/bin/bash", "-l")
	} else {
		return nil, nil, fmt.Errorf("resource type %q cannot host an interactive terminal", resource.Type)
	}
	command := exec.Command("ssh", append(args, jobArgs...)...)
	cleanup := func() {
		if jobName == "" {
			return
		}
		cancelArgs := append(append([]string{}, args...), "scancel", "--name="+jobName)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = exec.CommandContext(ctx, "ssh", cancelArgs...).Run()
	}
	return command, cleanup, nil
}

type terminalHandle struct {
	file    *os.File
	command *exec.Cmd
	cleanup func()
	once    sync.Once
}

func (h *terminalHandle) Read(buffer []byte) (int, error)  { return h.file.Read(buffer) }
func (h *terminalHandle) Write(buffer []byte) (int, error) { return h.file.Write(buffer) }
func (h *terminalHandle) Resize(rows, columns uint16) error {
	return pty.Setsize(h.file, &pty.Winsize{Rows: rows, Cols: columns})
}
func (h *terminalHandle) Close() (err error) {
	h.once.Do(func() {
		_ = h.file.Close()
		if h.command.Process != nil {
			_ = h.command.Process.Kill()
		}
		err = h.command.Wait()
		if h.cleanup != nil {
			h.cleanup()
		}
	})
	return err
}
