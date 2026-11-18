package local

import (
	"os"
	"runtime"
	"strings"
)

func nativeEnvironment() map[string]any {
	result := map[string]any{
		"architecture":   runtime.GOARCH,
		"cpuCapacity":    runtime.NumCPU(),
		"executionStyle": "native",
	}
	if capacity, err := nativeCapacity(); err == nil {
		result["cpuCapacity"] = capacity.MaxCPUs
		result["memoryBytes"] = capacity.MaxMemoryBytes
	}
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			key, value, found := strings.Cut(line, ":")
			if found && strings.TrimSpace(key) == "model name" {
				result["cpuModel"] = strings.TrimSpace(value)
				break
			}
		}
	}
	return result
}
