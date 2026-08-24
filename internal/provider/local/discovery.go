package local

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
)

// Discovery performs one shell invocation in the local execution context.
// It deliberately avoids recursive filesystem scans and performance probes.
type Discovery struct{ executor runtimecommon.CommandExecutor }

func NewDiscovery(executors ...runtimecommon.CommandExecutor) *Discovery {
	executor := runtimecommon.CommandExecutor(runtimecommon.OSCommandExecutor{})
	if len(executors) > 0 && executors[0] != nil {
		executor = executors[0]
	}
	return &Discovery{executor: executor}
}

func (d *Discovery) DiscoverConnection(ctx context.Context, _ domain.EnvironmentConnection) (ports.ConnectionDiscovery, error) {
	script := strings.Join([]string{
		`printf 'arch='; uname -m; printf 'cpu='; getconf _NPROCESSORS_ONLN`,
		`awk '/MemTotal/ {printf "memKiB=%s\n", $2}' /proc/meminfo`,
		`df -Pk / | awk 'NR==2 {printf "diskTotalKiB=%s\ndiskAvailableKiB=%s\n", $2, $4}'`,
		`for runtime in docker podman apptainer singularity; do if command -v "$runtime" >/dev/null 2>&1; then printf 'runtime=%s\n' "$runtime"; fi; done`,
	}, "; ")
	output, err := d.executor.Run(ctx, "/bin/sh", []string{"-c", script}, nil)
	if err != nil {
		return ports.ConnectionDiscovery{}, fmt.Errorf("local discovery: %w", err)
	}
	metadata := map[string]any{"mountPath": "/", "containerRuntimes": []string{}}
	runtimes := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "arch":
			metadata["architecture"] = value
		case "cpu":
			metadata["cpuCores"], _ = strconv.Atoi(value)
		case "memKiB":
			metadata["memoryBytes"] = kibibytes(value)
		case "diskTotalKiB":
			metadata["diskTotalBytes"] = kibibytes(value)
		case "diskAvailableKiB":
			metadata["diskAvailableBytes"] = kibibytes(value)
		case "runtime":
			runtimes = append(runtimes, value)
		}
	}
	metadata["containerRuntimes"] = runtimes
	return ports.ConnectionDiscovery{Available: true, Metadata: metadata}, nil
}

func kibibytes(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed * 1024
}
