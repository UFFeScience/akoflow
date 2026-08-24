package slurm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
)

// ConnectionProber verifies the scheduler through the same SSH route used by
// the runtime. It intentionally checks Slurm itself rather than only opening
// a TCP connection to the login node.
type ConnectionProber struct{ executor runtimecommon.CommandExecutor }

const defaultProbeTimeout = 15 * time.Second

func NewConnectionProber(executor runtimecommon.CommandExecutor) *ConnectionProber {
	if executor == nil {
		executor = runtimecommon.OSCommandExecutor{}
	}
	return &ConnectionProber{executor: executor}
}

func (p *ConnectionProber) Probe(ctx context.Context, connection domain.EnvironmentConnection) ports.ConnectionHealth {
	if connection.Type != domain.ConnectionSSH && connection.Type != domain.ConnectionAgent && connection.Type != domain.ConnectionLocal {
		return ports.ConnectionHealth{Message: fmt.Sprintf("unsupported SLURM connection type %q", connection.Type)}
	}
	executor := p.executor
	if connection.Type == domain.ConnectionSSH {
		executor = runtimecommon.SSHCommandExecutor{Executor: executor, Endpoint: connection.Endpoint,
			Username: connection.Username, Port: configInt(connection.Configuration, "port"),
			IdentityFile:   credentialFile(connection.CredentialRef),
			ProxyCommand:   configString(connection.Configuration, "proxyCommand"),
			HostKeyAlias:   configString(connection.Configuration, "hostKeyAlias"),
			KnownHostsFile: knownHostsFile(connection),
			ForwardAgent:   configBool(connection.Configuration, "forwardAgent", false)}
	}
	probeContext, cancel := context.WithTimeout(ctx, defaultProbeTimeout)
	defer cancel()
	output, err := executor.Run(probeContext, "sinfo", []string{"--noheader", "--format=%P"}, nil)
	if err != nil {
		return ports.ConnectionHealth{Message: fmt.Sprintf("SLURM is unreachable: %v", err)}
	}
	partitions := strings.TrimSpace(string(output))
	if partitions == "" {
		return ports.ConnectionHealth{Message: "SLURM responded without visible partitions"}
	}
	return ports.ConnectionHealth{Healthy: true, Message: "SLURM is reachable: " + strings.Split(partitions, "\n")[0]}
}
