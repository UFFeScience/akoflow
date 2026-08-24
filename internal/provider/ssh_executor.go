package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SSHCommandExecutor runs a command on a login node while preserving the
// CommandExecutor contract used by the SLURM adapter. Authentication is left
// to SSH configuration, an agent, or a configured identity file; no secret is
// stored in the environment definition.
type SSHCommandExecutor struct {
	Executor       CommandExecutor
	Endpoint       string
	Username       string
	Port           int
	IdentityFile   string
	ProxyCommand   string
	HostKeyAlias   string
	KnownHostsFile string
	ForwardAgent   bool
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
		sshArgs = append(sshArgs, "-o", "ProxyCommand="+e.ProxyCommand)
	}
	if e.HostKeyAlias != "" {
		sshArgs = append(sshArgs, "-o", "HostKeyAlias="+e.HostKeyAlias)
	}
	if e.ForwardAgent {
		sshArgs = append(sshArgs, "-A")
	}
	// ssh serializes the remote command as shell text. Quote every argument so
	// a script passed to `sh -c` remains one argument on the login node (and so
	// paths or values with spaces cannot change the remote command structure).
	remote := make([]string, 0, len(args)+1)
	remote = append(remote, shellQuote(name))
	for _, arg := range args {
		remote = append(remote, shellQuote(arg))
	}
	sshArgs = append(sshArgs, target, strings.Join(remote, " "))
	return e.Executor.Run(ctx, "ssh", sshArgs, input)
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\\"'\\\"'") + "'"
}
