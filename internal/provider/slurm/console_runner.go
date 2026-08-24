package slurm

import (
	"context"
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainconsole "github.com/UFFeScience/akoflow/internal/domain/console"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
)

type ConsoleRunner struct{ Executor runtimecommon.CommandExecutor }

func (r ConsoleRunner) RunConsoleCommand(ctx context.Context, connection domain.EnvironmentConnection, resource domain.Resource, command domainconsole.Command) (string, string, int, string, error) {
	if connection.Type != domain.ConnectionSSH && connection.Type != domain.ConnectionAgent && connection.Type != domain.ConnectionLocal {
		return "", "", 1, "", fmt.Errorf("console commands are not supported for connection type %q", connection.Type)
	}
	executor := r.Executor
	if executor == nil {
		executor = runtimecommon.OSCommandExecutor{}
	}
	if connection.Type == domain.ConnectionSSH {
		executor = runtimecommon.SSHCommandExecutor{Executor: executor, Endpoint: connection.Endpoint,
			Username: connection.Username, Port: configInt(connection.Configuration, "port"), IdentityFile: credentialFile(connection.CredentialRef),
			ProxyCommand: configString(connection.Configuration, "proxyCommand"), HostKeyAlias: configString(connection.Configuration, "hostKeyAlias"),
			KnownHostsFile: knownHostsFile(connection),
			ForwardAgent:   configBool(connection.Configuration, "forwardAgent", false)}
	}
	script := consoleScript(resource, command)
	output, err := executor.Run(ctx, "/bin/sh", []string{"-s"}, []byte(script))
	if err != nil {
		return string(output), "", 1, "", err
	}
	return string(output), "", 0, "", nil
}

func consoleScript(resource domain.Resource, command domainconsole.Command) string {
	var script strings.Builder
	script.WriteString("#!/bin/sh\nset -eu\n")
	for key, value := range command.Environment {
		script.WriteString("export " + shellToken(key) + "=" + shellQuote(value) + "\n")
	}
	if command.WorkingDirectory != "" {
		script.WriteString("cd " + shellQuote(command.WorkingDirectory) + "\n")
	}
	if resource.ExecutionTarget == domain.ExecutionTargetDirect {
		script.WriteString("exec /bin/sh -lc " + shellQuote(command.Command) + "\n")
		return script.String()
	}
	script.WriteString("exec srun")
	if resource.Type == domain.ResourceHPCPartition {
		script.WriteString(" --partition=" + shellToken(resource.ProviderID))
	}
	if resource.Type == domain.ResourceHPCMachine {
		script.WriteString(" --nodelist=" + shellToken(resource.ProviderID))
	}
	if command.CPUCores > 0 {
		script.WriteString(fmt.Sprintf(" --cpus-per-task=%d", command.CPUCores))
	}
	if command.MemoryBytes > 0 {
		script.WriteString(fmt.Sprintf(" --mem=%dM", (command.MemoryBytes+(1<<20)-1)/(1<<20)))
	}
	script.WriteString(fmt.Sprintf(" --time=%d", max(1, (command.TimeoutSeconds+59)/60)))
	script.WriteString(" /bin/sh -lc " + shellQuote(command.Command) + "\n")
	return script.String()
}
