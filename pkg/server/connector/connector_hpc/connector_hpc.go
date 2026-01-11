package connector_hpc

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"sync"
	"syscall"

	"github.com/ovvesley/akoflow/pkg/server/entities/runtime_entity"
)

type ConnectorHPCRuntime struct {
	Runtime runtime_entity.Runtime
}

func New() IConnectorHPCRuntime {
	return &ConnectorHPCRuntime{}
}

func (c *ConnectorHPCRuntime) SetRuntime(runtime runtime_entity.Runtime) *ConnectorHPCRuntime {
	c.Runtime = runtime
	return c
}

type IConnectorHPCRuntime interface {
	RunCommand(command string, args ...string) (string, error)
	RunCommandWithOutput(command string, args ...string) (string, error)
	RunCommandWithOutputRemote(command string, args ...string) (string, error)
	IsVPNConnected() (bool, error)
	ExecuteMultiplesCommand(commands []string)
	SetRuntime(runtime runtime_entity.Runtime) *ConnectorHPCRuntime
	BuildRemoteCommand(runtime runtime_entity.Runtime, command string) (string, error)
}

func (c *ConnectorHPCRuntime) RunCommandWithOutputRemote(command string, args ...string) (string, error) {
	fmt.Printf("Executing command: %s %v\n", command, args)

	shell := getAvailableShell()

	remoteCommand, err := c.BuildRemoteCommand(c.Runtime, command)
	if err != nil {
		return "", err
	}

	fullCommand := append([]string{"-c", remoteCommand}, args...)
	cmd := exec.Command(shell, fullCommand...)
	output, err := cmd.CombinedOutput()

	println(string(output))
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w", err)
	}

	return string(output), nil
}

func (c *ConnectorHPCRuntime) handleCreateSSHKey(privateKey string, publicKey string, sshConfig string) error {
	if privateKey == "" || publicKey == "" || sshConfig == "" {
		return nil
	}

	privateKeyDecoded, err := decodeBase64(privateKey)
	if err != nil {
		return err
	}

	publicKeyDecoded, err := decodeBase64(publicKey)
	if err != nil {
		return err
	}

	sshConfigDecoded, err := decodeBase64(sshConfig)
	if err != nil {
		return err
	}

	privateKeyFile, err := writeTempSSHKey(privateKeyDecoded, "id_rsa")
	if err != nil {
		return err
	}

	publicKeyFile, err := writeTempSSHKey(publicKeyDecoded, "id_rsa.pub")
	if err != nil {
		removeTempSSHKey(privateKeyFile)
		return err
	}

	_, err = writeTempSSHKey(sshConfigDecoded, "config")

	if err != nil {
		removeTempSSHKey(privateKeyFile)
		removeTempSSHKey(publicKeyFile)
		return err
	}

	return nil
}

func (c ConnectorHPCRuntime) BuildRemoteCommand(runtime runtime_entity.Runtime, command string) (string, error) {
	username := runtime.GetCurrentRuntimeMetadata("USER")
	hostname := runtime.GetCurrentRuntimeMetadata("HOST_CLUSTER")
	sshKeyPrivateKey := runtime.GetCurrentRuntimeMetadata("SSHKEYPRIVK")
	sshKeyPublicKey := runtime.GetCurrentRuntimeMetadata("SSHKEYPUBLK")
	sshConfig := runtime.GetCurrentRuntimeMetadata("SSHCONFIG")
	password := runtime.GetCurrentRuntimeMetadata("PASSWORD")

	// create .ssh/id_rsa file with the private key
	c.handleCreateSSHKey(sshKeyPrivateKey, sshKeyPublicKey, sshConfig)

	if sshKeyPrivateKey != "" && sshKeyPublicKey != "" && sshConfig != "" {
		return fmt.Sprintf("ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=10 %s@%s '%s'", username, hostname, command), nil
	} else if password != "" {
		return fmt.Sprintf("sshpass -p '%s' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=10 %s@%s '%s'", password, username, hostname, command), nil
	}

	return "", fmt.Errorf("no authentication method provided")
}

func decodeBase64(encoded string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}
	return string(decoded), nil
}

func writeTempSSHKey(key string, filename string) (string, error) {
	path := "/root/.ssh/"

	keyFile := path + filename

	_, err := exec.Command("bash", "-c", fmt.Sprintf("test -f %s", keyFile)).Output()
	if err == nil {
		return keyFile, nil
	}

	err = exec.Command("bash", "-c", fmt.Sprintf("echo '%s' > %s && chmod 600 %s", key, keyFile, keyFile)).Run()
	if err != nil {
		return "", fmt.Errorf("failed to write SSH key to file: %w", err)
	}

	return keyFile, nil
}

func removeTempSSHKey(file string) {
	exec.Command("bash", "-c", fmt.Sprintf("rm -f %s", file)).Run()
}

func (s *ConnectorHPCRuntime) ExecuteMultiplesCommand(commands []string) {
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

			fmt.Printf("Executing command: %s %v\n", command, fullCommand)

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

func (c *ConnectorHPCRuntime) RunCommand(command string, args ...string) (string, error) {
	return executeCommand(command, args...)
}

func (c *ConnectorHPCRuntime) RunCommandWithOutput(command string, args ...string) (string, error) {
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

func (c *ConnectorHPCRuntime) IsVPNConnected() (bool, error) {
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
	cmd.SysProcAttr = &syscall.SysProcAttr{}
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
