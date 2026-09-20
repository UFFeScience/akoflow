package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// SSHCommandExecutor runs a command on a login node while preserving the
// CommandExecutor contract used by the SLURM adapter. Authentication is left
// to SSH configuration, an agent, or a configured identity file; no secret is
// stored in the environment definition.
type SSHCommandExecutor struct {
	Executor       CommandExecutor
	ConnectionID   string
	Endpoint       string
	Username       string
	Port           int
	IdentityFile   string
	ProxyCommand   string
	HostKeyAlias   string
	KnownHostsFile string
	ForwardAgent   bool
}

// NewSSHCommandExecutor builds the canonical daemon-side SSH transport for an
// environment connection. Every caller gets the same proxy, credential,
// host-key and agent-forwarding behavior.
func NewSSHCommandExecutor(executor CommandExecutor, connection domain.EnvironmentConnection) SSHCommandExecutor {
	if executor == nil {
		executor = OSCommandExecutor{}
	}
	knownHosts := connectionString(connection.Configuration, "knownHostsFile")
	if knownHosts == "" {
		knownHosts = filepath.Join("storage", "credentials", "ssh", "known_hosts")
	}
	identity := ""
	if strings.HasPrefix(connection.CredentialRef, "file:") {
		identity = strings.TrimSpace(strings.TrimPrefix(connection.CredentialRef, "file:"))
	}
	return SSHCommandExecutor{
		Executor: executor, ConnectionID: connection.ID, Endpoint: connection.Endpoint, Username: connection.Username,
		Port:           connectionInt(connection.Configuration, "port"),
		IdentityFile:   identity,
		ProxyCommand:   connectionString(connection.Configuration, "proxyCommand"),
		HostKeyAlias:   connectionString(connection.Configuration, "hostKeyAlias"),
		KnownHostsFile: knownHosts,
		ForwardAgent:   connectionBool(connection.Configuration, "forwardAgent"),
	}
}

func (e SSHCommandExecutor) Run(ctx context.Context, name string, args []string, input []byte) ([]byte, error) {
	if e.Executor == nil {
		return nil, fmt.Errorf("SSH command executor is required")
	}
	if strings.TrimSpace(e.Endpoint) == "" {
		return nil, fmt.Errorf("SSH endpoint is required")
	}
	target := strings.TrimSpace(e.Endpoint)
	if e.Username != "" && !strings.Contains(target, "@") {
		target = e.Username + "@" + target
	}
	// Health checks and workers must never block indefinitely on an unreachable
	// login node or a ProxyCommand that keeps a pipe open after its parent dies.
	sshArgs := make([]string, 0, len(args)+14)
	sshArgs = append(sshArgs, "-o", "BatchMode=yes", "-o", "ConnectionAttempts=1", "-o", "ConnectTimeout=10", "-o", "CheckHostIP=no")
	if e.KnownHostsFile != "" {
		if err := os.MkdirAll(filepath.Dir(e.KnownHostsFile), 0o700); err != nil {
			return nil, fmt.Errorf("create SSH known-hosts directory: %w", err)
		}
		sshArgs = append(sshArgs, "-o", "UserKnownHostsFile="+e.KnownHostsFile, "-o", "StrictHostKeyChecking=accept-new")
	}
	if e.Port > 0 {
		sshArgs = append(sshArgs, "-p", strconv.Itoa(e.Port))
	}
	if e.IdentityFile != "" {
		sshArgs = append(sshArgs, "-i", e.IdentityFile)
	}
	if e.ProxyCommand != "" {
		sshArgs = append(sshArgs, "-o", "ProxyCommand="+ProxyCommandWithKnownHosts(e.ProxyCommand, e.KnownHostsFile, e.IdentityFile))
	}
	if e.HostKeyAlias != "" {
		sshArgs = append(sshArgs, "-o", "HostKeyAlias="+e.HostKeyAlias)
	}
	if e.ForwardAgent {
		sshArgs = append(sshArgs, "-A")
	}
	multiplex, controlPath, err := SSHMultiplexArguments(SSHSessionKey{
		ConnectionID: e.ConnectionID, Username: e.Username, Host: e.Endpoint, Port: e.Port,
		IdentityFile: e.IdentityFile, ProxyCommand: e.ProxyCommand, KnownHostsFile: e.KnownHostsFile,
		HostKeyAlias: e.HostKeyAlias, ForwardAgent: e.ForwardAgent,
	})
	if err != nil {
		return nil, err
	}
	sshArgs = append(sshArgs, multiplex...)
	// ssh serializes the remote command as shell text. Quote every argument so
	// a script passed to `sh -c` remains one argument on the login node (and so
	// paths or values with spaces cannot change the remote command structure).
	remote := make([]string, 0, len(args)+1)
	remote = append(remote, shellQuote(name))
	for _, arg := range args {
		remote = append(remote, shellQuote(arg))
	}
	sshArgs = append(sshArgs, target, strings.Join(remote, " "))
	releaseChannel, err := AcquireSSHChannel(ctx, controlPath)
	if err != nil {
		return nil, fmt.Errorf("wait for SSH channel: %w", err)
	}
	defer releaseChannel()
	unlock := sharedSSHSessions.lockForCreation(controlPath)
	output, runErr := e.Executor.Run(ctx, "ssh", sshArgs, input)
	unlock()
	if !isSSHControlSocketError(runErr, output) || controlPath == "" {
		return output, runErr
	}
	unlockRecovery := sharedSSHSessions.lock(controlPath)
	defer unlockRecovery()
	if removeErr := os.Remove(controlPath); removeErr != nil && !os.IsNotExist(removeErr) {
		return output, runErr
	}
	return e.Executor.Run(ctx, "ssh", sshArgs, input)
}

// A ProxyCommand starts its own SSH process. Give that hop the same
// daemon-managed known_hosts policy as the destination so first-time gateway
// connections can be recorded during the explicit connection test.
func ProxyCommandWithKnownHosts(command, knownHosts, identityFile string) string {
	command = strings.TrimSpace(command)
	if knownHosts == "" || !strings.HasPrefix(command, "ssh ") {
		return command
	}
	options := "-o UserKnownHostsFile=" + shellQuote(knownHosts) + " -o StrictHostKeyChecking=accept-new "
	if identityFile != "" {
		options += "-i " + shellQuote(identityFile) + " "
	}
	return "ssh " + options + strings.TrimSpace(strings.TrimPrefix(command, "ssh "))
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func connectionString(configuration map[string]any, key string) string {
	value, _ := configuration[key].(string)
	return strings.TrimSpace(value)
}

func connectionInt(configuration map[string]any, key string) int {
	switch value := configuration[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case string:
		parsed, _ := strconv.Atoi(value)
		return parsed
	default:
		return 0
	}
}

func connectionBool(configuration map[string]any, key string) bool {
	value, _ := configuration[key].(bool)
	return value
}
