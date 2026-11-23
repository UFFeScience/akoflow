package local

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
	runtimecommon "github.com/UFFeScience/akoflow/internal/provider"
)

func detectCapacity(ctx context.Context, executor runtimecommon.CommandExecutor) (*domain.LocalCapacity, error) {
	output, err := executor.Run(ctx, "docker", []string{"info", "--format", "{{.NCPU}} {{.MemTotal}}"}, nil)
	var cpus int
	var memory int64
	if err == nil {
		if _, scanErr := fmt.Sscan(string(output), &cpus, &memory); scanErr == nil && cpus > 0 && memory > 0 {
			return domain.NewLocalCapacity(cpus, memory, "docker")
		}
	}
	return nativeCapacity()
}

func nativeCapacity() (*domain.LocalCapacity, error) {
	cpus := runtime.NumCPU()
	memory, err := totalNativeMemory()
	if err != nil {
		return nil, err
	}
	memory = boundedMemory(memory, "/sys/fs/cgroup/memory.max")
	memory = boundedMemory(memory, "/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if quota, err := os.ReadFile("/sys/fs/cgroup/cpu.max"); err == nil {
		var count, period int64
		if _, err := fmt.Sscan(string(quota), &count, &period); err == nil && count > 0 && period > 0 {
			cpus = min(cpus, max(1, int(count/period)))
		}
	}
	return domain.NewLocalCapacity(cpus, memory, "native")
}

func totalNativeMemory() (int64, error) {
	var memory int64
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, fmt.Errorf("detect local memory: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			var kib int64
			fmt.Sscan(strings.TrimPrefix(line, "MemTotal:"), &kib)
			memory = kib * 1024
			break
		}
	}
	return memory, nil
}

func boundedMemory(total int64, path string) int64 {
	raw, err := os.ReadFile(path)
	if err != nil {
		return total
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err == nil && value > 0 {
		return min(total, value)
	}
	return total
}
