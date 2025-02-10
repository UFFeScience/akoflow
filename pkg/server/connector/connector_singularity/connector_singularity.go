package connector_singularity

import (
	"fmt"
	"os/exec"
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
	output, err := cmd.CombinedOutput()

	fmt.Printf("Command output: %s\n", output)

	if err != nil {
		fmt.Printf("Error executing command: %v\n", err)
		return "", err
	}

	return string(output), nil
}

func getAvailableShell() string {
	if _, err := exec.LookPath("bash"); err == nil {
		return "bash"
	}
	return "sh"
}
