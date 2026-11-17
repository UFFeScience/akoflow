package build

import (
	"context"
	"strings"

	"github.com/UFFeScience/akoflow/internal/provider"
)

type localInspectionKey struct{}

func LocalInspectionOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, localInspectionKey{}, true)
}

func (m Manager) inspectRegistryPlatforms(ctx context.Context, image string, result map[string]any) {
	if ctx.Value(localInspectionKey{}) == true {
		return
	}
	platforms, reference := provider.ResolveOCIPlatforms(ctx, image)
	if len(platforms) > 0 {
		result["platforms"] = platforms
		result["platformsRegistryReference"] = reference
		return
	}
	if platforms = provider.RegistryPlatforms(ctx, m.Executor.Runner, image); len(platforms) > 0 {
		result["platforms"] = platforms
		if !strings.Contains(image, "@sha256:") {
			result["platformsObservedFromTag"] = true
		}
	}
}

func inspectedRegistryReference(reference string, image map[string]any) string {
	if strings.Contains(reference, "@") {
		return reference
	}
	repository := func(value string) string {
		value = strings.SplitN(value, "@", 2)[0]
		if i := strings.LastIndex(value, ":"); i > strings.LastIndex(value, "/") {
			value = value[:i]
		}
		value = strings.TrimPrefix(strings.TrimPrefix(value, "docker.io/"), "index.docker.io/")
		return strings.TrimPrefix(value, "library/")
	}
	digests, _ := image["RepoDigests"].([]any)
	config, _ := image["Id"].(string)
	for _, value := range digests {
		digest, _ := value.(string)
		if strings.Contains(digest, "@sha256:") && !strings.HasSuffix(digest, "@"+config) && repository(digest) == repository(reference) {
			return digest
		}
	}
	return reference
}
