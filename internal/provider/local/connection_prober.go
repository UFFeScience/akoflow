package local

import (
	"context"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
)

type ConnectionProber struct{ executor runtimecommon.CommandExecutor }

func NewConnectionProber(executors ...runtimecommon.CommandExecutor) *ConnectionProber {
	executor := runtimecommon.CommandExecutor(runtimecommon.OSCommandExecutor{})
	if len(executors) > 0 && executors[0] != nil {
		executor = executors[0]
	}
	return &ConnectionProber{executor: executor}
}

func (p *ConnectionProber) Probe(ctx context.Context, _ domain.EnvironmentConnection) ports.ConnectionHealth {
	if _, err := p.executor.Run(ctx, "/bin/sh", []string{"-c", "true"}, nil); err != nil {
		return ports.ConnectionHealth{Message: fmt.Sprintf("local execution target is unavailable: %v", err)}
	}
	return ports.ConnectionHealth{Healthy: true, Message: "local engine container accepted an execution probe"}
}
