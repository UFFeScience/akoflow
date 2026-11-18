package local

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

func dockerEnvironment(ctx context.Context, compatibilityMode bool) map[string]any {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	result := map[string]any{}
	output, err := exec.CommandContext(ctx, "docker", "info", "--format", "{{.Architecture}} {{.NCPU}} {{.MemTotal}} {{.ServerVersion}}").Output()
	if err != nil {
		return result
	}
	var architecture, version string
	var cpus int
	var memory int64
	if _, err := fmt.Sscan(string(output), &architecture, &cpus, &memory, &version); err != nil {
		return result
	}
	result["architecture"] = architecture
	result["cpuCores"] = cpus
	result["memoryBytes"] = memory
	result["version"] = version
	if compatibilityMode {
		result["executionStyle"] = "emulated"
	} else if architecture == runtime.GOARCH || architecture == "x86_64" && runtime.GOARCH == "amd64" || architecture == "aarch64" && runtime.GOARCH == "arm64" {
		result["executionStyle"] = "native"
	}
	return result
}
