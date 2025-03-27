package connector_sdumont

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type ConnectorSDumont struct {
}

func New() IConnectorSDumont {
	return &ConnectorSDumont{}
}

type IConnectorSDumont interface {
	RunCommand(command string, args ...string) (string, error)
	RunCommandWithOutput(command string, args ...string) (string, error)
	RunCommandWithOutputRemote(command string, args ...string) (string, error)
	IsVPNConnected() (bool, error)
	ExecuteMultiplesCommand(commands []string)
}

func (c *ConnectorSDumont) RunCommandWithOutputRemote(command string, args ...string) (string, error) {
	fmt.Printf("Executing command: %s %v\n", command, args)

	shell := getAvailableShell()

	password := os.Getenv("SDUMONT_PASSWORD")
	username := os.Getenv("SDUMONT_USER")
	hostname := os.Getenv("SDUMONT_HOST_CLUSTER")

	command = fmt.Sprintf("sshpass -p '%s' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=10 %s@%s '%s'", password, username, hostname, command)

	fullCommand := append([]string{"-c", command}, args...)
	cmd := exec.Command(shell, fullCommand...)
	output, err := cmd.CombinedOutput()
	println(string(output))
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w", err)
	}

	return string(output), nil
}

func (s *ConnectorSDumont) ExecuteMultiplesCommand(commands []string) {
	var wg sync.WaitGroup

	responses := make(chan string, len(commands)) // Create a channel to receive the responses

	for _, command := range commands {
		wg.Add(1)
		go func(command string) {
			defer wg.Done()

			shell := getAvailableShell()

			fullCommand := append([]string{"-c", command})
			cmd := exec.Command(shell, fullCommand...)
			output, err := cmd.CombinedOutput()

			if err != nil {
				fmt.Printf("failed to execute command: %s\n", err)
			}

			fmt.Printf("Output: %s\n", output)

			responses <- string(output)

		}(command)
	}

	wg.Wait()

	close(responses)

	for response := range responses {
		fmt.Printf("Response: %s\n", response)
	}
}

func (c *ConnectorSDumont) RunCommand(command string, args ...string) (string, error) {
	return executeCommand(command, args...)
}

func (c *ConnectorSDumont) RunCommandWithOutput(command string, args ...string) (string, error) {
	fmt.Printf("Executing command: %s %v\n", command, args)

	shell := getAvailableShell()

	fullCommand := append([]string{"-c", command}, args...)
	cmd := exec.Command(shell, fullCommand...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w", err)
	}

	return string(output), nil

}

func (c *ConnectorSDumont) IsVPNConnected() (bool, error) {
	output, err := c.RunCommandWithOutput("ps aux | grep vpnc")
	if err != nil {
		return false, err
	}

	if len(output) > 0 {
		return true, nil
	}

	return false, nil
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
