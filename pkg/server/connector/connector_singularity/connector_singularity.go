package connector_singularity

import (
	"fmt"
	"os/exec"
	"syscall"
)

type ConnectorSingularity struct {
}

func New() IConnectorSingularity {
	return &ConnectorSingularity{}
}

type IConnectorSingularity interface {
	RunCommand(command string, args ...string) (string, error)
}

func (c *ConnectorSingularity) RunCommand(command string, args ...string) (string, error) {
	return executeCommand(command, args...)
}

func executeCommand(command string, args ...string) (string, error) {
	fmt.Printf("Executing command: %s %v\n", command, args)

	shell := getAvailableShell()
	fullCommand := append([]string{"-c", command}, args...)
	cmd := exec.Command(shell, fullCommand...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	pid := cmd.Process.Pid
	fmt.Printf("Started process with PID: %d\n", pid)

	return fmt.Sprintf("%d", pid), nil
}

func getAvailableShell() string {
	if _, err := exec.LookPath("bash"); err == nil {
		return "bash"
	}
	return "sh"
}
