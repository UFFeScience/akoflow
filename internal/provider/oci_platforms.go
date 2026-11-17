package provider

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// RegistryPlatforms reads the registry manifest without pulling the image.
func RegistryPlatforms(ctx context.Context, runner CommandExecutor, image string) []string {
	if runner == nil || image == "" || strings.HasPrefix(image, "-") || strings.ContainsAny(image, " \r\n\t") {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	data, err := runner.Run(ctx, "docker", []string{"manifest", "inspect", "--verbose", image}, nil)
	if err != nil {
		return nil
	}
	type manifest struct {
		Descriptor struct {
			Platform struct {
				OS           string
				Architecture string
			}
		}
	}
	var manifests []manifest
	if json.Unmarshal(data, &manifests) != nil {
		var single manifest
		if json.Unmarshal(data, &single) != nil {
			return nil
		}
		manifests = []manifest{single}
	}
	seen := map[string]bool{}
	var platforms []string
	for _, item := range manifests {
		p := item.Descriptor.Platform
		if p.OS != "linux" || (p.Architecture != "arm64" && p.Architecture != "amd64") {
			continue
		}
		platform := p.OS + "/" + p.Architecture
		if !seen[platform] {
			seen[platform] = true
			platforms = append(platforms, platform)
		}
	}
	sort.Strings(platforms)
	return platforms
}
