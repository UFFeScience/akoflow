package local

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"strings"

	"github.com/UFFeScience/akoflow/internal/provider"
)

const compatibilityWarning = "platformCompatibilityWarning"

func startContainerForPlatform(ctx context.Context, args []string, image string) ([]byte, string, error) {
	return startContainerWithRegistry(ctx, args, image, func(ctx context.Context, image string) []string {
		if platforms, _ := provider.ResolveOCIPlatforms(ctx, image); len(platforms) > 0 {
			return platforms
		}
		return provider.RegistryPlatforms(ctx, provider.OSCommandExecutor{}, image)
	})
}

func startContainerWithRegistry(ctx context.Context, args []string, image string, lookup func(context.Context, string) []string) ([]byte, string, error) {
	native := "linux/" + runtime.GOARCH
	platformArgs := func(platform string) []string {
		return append([]string{"run", "--platform", platform}, args[1:]...)
	}
	output, err := exec.CommandContext(ctx, "docker", platformArgs(native)...).CombinedOutput()
	if err == nil || ctx.Err() != nil || !dockerPlatformMismatch(string(output)) {
		return output, "", err
	}
	var platforms []string
	if !strings.HasPrefix(image, "sha256:") {
		platforms = lookup(ctx, image)
	}
	if slices.Contains(platforms, native) {
		if pullOutput, pullErr := exec.CommandContext(ctx, "docker", "pull", "--platform", native, image).CombinedOutput(); pullErr != nil {
			return pullOutput, "", pullErr
		}
		output, err = exec.CommandContext(ctx, "docker", platformArgs(native)...).CombinedOutput()
		return output, "", err
	}
	if len(platforms) == 0 {
		local, inspectErr := exec.CommandContext(ctx, "docker", "image", "inspect", "--format", "{{.Os}}/{{.Architecture}}", image).Output()
		if platform := strings.TrimSpace(string(local)); inspectErr == nil && (platform == "linux/amd64" || platform == "linux/arm64") && platform != native {
			platforms = []string{platform}
		}
	}
	if len(platforms) != 1 {
		return output, "", err
	}
	target := platforms[0]
	warning := fmt.Sprintf("Compatibility mode: running image for %s on %s with Docker emulation.", target, native)
	output, err = exec.CommandContext(ctx, "docker", platformArgs(target)...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("%s Check that Docker supports emulation for %s: %w", warning, target, err)
	}
	return output, warning, err
}

func dockerPlatformMismatch(output string) bool {
	output = strings.ToLower(output)
	return strings.Contains(output, "no matching manifest") ||
		strings.Contains(output, "does not provide the specified platform") ||
		(strings.Contains(output, "platform") && strings.Contains(output, "does not match"))
}
