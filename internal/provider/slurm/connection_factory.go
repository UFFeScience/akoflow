package slurm

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
)

// ConnectionFactory creates a SLURM adapter tied to one login-node
// connection. It lets a single engine submit independently to multiple HPC
// systems rather than inheriting one process-wide sbatch target.
type ConnectionFactory struct {
	Executor               runtimecommon.CommandExecutor
	DefaultScriptDirectory string
}

func (ConnectionFactory) Driver() domain.RuntimeDriver { return domain.RuntimeDriverSlurm }

func (f ConnectionFactory) Build(runtime domain.EnvironmentRuntime, connection domain.EnvironmentConnection) (ports.RuntimeAdapter, error) {
	if connection.Type != domain.ConnectionSSH && connection.Type != domain.ConnectionAgent && connection.Type != domain.ConnectionLocal {
		return nil, fmt.Errorf("connection %q is %q, SLURM runtime requires ssh, agent, or local connection", connection.ID, connection.Type)
	}
	executor := f.Executor
	if executor == nil {
		executor = runtimecommon.OSCommandExecutor{}
	}
	remote := connection.Type == domain.ConnectionSSH && strings.TrimSpace(connection.Endpoint) != ""
	if remote {
		executor = runtimecommon.SSHCommandExecutor{
			Executor: executor, Endpoint: connection.Endpoint, Username: connection.Username,
			Port:           configInt(connection.Configuration, "port"),
			IdentityFile:   credentialFile(connection.CredentialRef),
			ProxyCommand:   configString(connection.Configuration, "proxyCommand"),
			HostKeyAlias:   configString(connection.Configuration, "hostKeyAlias"),
			KnownHostsFile: knownHostsFile(connection),
			ForwardAgent:   configBool(connection.Configuration, "forwardAgent", false),
		}
	}
	scriptDirectory := configString(runtime.Configuration, "scriptDirectory")
	if scriptDirectory == "" {
		scriptDirectory = configString(connection.Configuration, "scriptDirectory")
	}
	if scriptDirectory == "" {
		scriptDirectory = f.DefaultScriptDirectory
	}
	if scriptDirectory == "" {
		return nil, fmt.Errorf("runtime %q needs a script directory", runtime.ID)
	}
	partition := configString(runtime.Configuration, "partition")
	if partition == "" {
		partition = configString(connection.Configuration, "partition")
	}
	return NewWithConfig(executor, Config{Partition: partition, ScriptDirectory: scriptDirectory,
		SubmitFromStdin: remote}), nil
}

func knownHostsFile(connection domain.EnvironmentConnection) string {
	if value := configString(connection.Configuration, "knownHostsFile"); value != "" {
		return value
	}
	return filepath.Join("storage", "credentials", "ssh", "known_hosts")
}

func configString(configuration map[string]any, key string) string {
	if value, ok := configuration[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func configInt(configuration map[string]any, key string) int {
	switch value := configuration[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case string:
		parsed, _ := strconv.Atoi(value)
		return parsed
	default:
		return 0
	}
}

func configBool(configuration map[string]any, key string, fallback bool) bool {
	value, ok := configuration[key].(bool)
	if !ok {
		return fallback
	}
	return value
}

func credentialFile(reference string) string {
	if strings.HasPrefix(reference, "file:") {
		return strings.TrimSpace(strings.TrimPrefix(reference, "file:"))
	}
	return ""
}
