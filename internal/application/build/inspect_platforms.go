package build

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

func (m Manager) inspectRegistryPlatforms(ctx context.Context, image string, result map[string]any) {
	host, _, qualified := strings.Cut(image, "/")
	if !qualified || !strings.Contains(host, ".") || !strings.Contains(image, "@sha256:") {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := m.Executor.Runner.Run(ctx, "docker", []string{"manifest", "inspect", image}, nil)
	if err != nil {
		return
	}
	var index struct {
		Manifests []struct {
			Platform struct {
				OS           string
				Architecture string
			}
		}
	}
	if json.Unmarshal(output, &index) != nil {
		return
	}
	seen := map[string]bool{}
	var platforms []string
	for _, descriptor := range index.Manifests {
		platform := descriptor.Platform
		if platform.OS != "linux" || (platform.Architecture != "amd64" && platform.Architecture != "arm64") {
			continue
		}
		name := platform.OS + "/" + platform.Architecture
		if !seen[name] {
			seen[name] = true
			platforms = append(platforms, name)
		}
	}
	if len(platforms) > 0 {
		sort.Strings(platforms)
		result["platforms"] = platforms
	}
}
