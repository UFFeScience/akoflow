package provider

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type registryPlatform struct {
	OS           string
	Architecture string
}

// RegistryPlatforms reads the manifest without pulling the image.
func RegistryPlatforms(ctx context.Context, runner CommandExecutor, image string) []string {
	if runner == nil || image == "" || strings.HasPrefix(image, "-") || strings.ContainsAny(image, " \r\n\t") {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	data, err := runner.Run(ctx, "docker", []string{"manifest", "inspect", image}, nil)
	if err != nil {
		return nil
	}
	var index struct {
		Manifests []struct{ Platform registryPlatform }
	}
	if json.Unmarshal(data, &index) != nil {
		return nil
	}
	if len(index.Manifests) > 0 {
		platforms := make([]registryPlatform, 0, len(index.Manifests))
		for _, descriptor := range index.Manifests {
			platforms = append(platforms, descriptor.Platform)
		}
		return supportedRegistryPlatforms(platforms)
	}
	data, err = runner.Run(ctx, "docker", []string{"manifest", "inspect", "--verbose", image}, nil)
	if err != nil {
		return nil
	}
	var entry struct {
		Descriptor struct{ Platform registryPlatform }
	}
	if json.Unmarshal(data, &entry) != nil {
		return nil
	}
	return supportedRegistryPlatforms([]registryPlatform{entry.Descriptor.Platform})
}

func supportedRegistryPlatforms(platforms []registryPlatform) []string {
	seen := map[string]bool{}
	var result []string
	for _, platform := range platforms {
		if platform.OS != "linux" || (platform.Architecture != "arm64" && platform.Architecture != "amd64") {
			continue
		}
		name := platform.OS + "/" + platform.Architecture
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}
