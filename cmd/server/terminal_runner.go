package main

import (
	"context"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/provider/kubernetes"
	"github.com/UFFeScience/akoflow/internal/provider/slurm"
)

type terminalRunner struct {
	kubernetes kubernetes.TerminalRunner
	slurm      slurm.TerminalRunner
}

func (r terminalRunner) StartInteractive(ctx context.Context, connection domain.EnvironmentConnection, resource domain.Resource) (ports.InteractiveTerminal, error) {
	if connection.Type == domain.ConnectionKubernetes {
		return r.kubernetes.StartInteractive(ctx, connection, resource)
	}
	if connection.Type == domain.ConnectionSSH || connection.Type == domain.ConnectionAgent || connection.Type == domain.ConnectionLocal {
		return r.slurm.StartInteractive(ctx, connection, resource)
	}
	return nil, fmt.Errorf("interactive terminal is not supported for connection type %q", connection.Type)
}
