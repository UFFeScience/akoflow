package local

import (
	"context"
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var dockerImageID = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func containerImageObservation(ctx context.Context, containerID, requested string) map[string]any {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	observation := map[string]any{"requestedReference": requested, "status": "unavailable"}
	output, err := exec.CommandContext(ctx, "docker", "container", "inspect", "--format", "{{.Image}}", containerID).Output()
	imageID := strings.TrimSpace(string(output))
	if err != nil || !dockerImageID.MatchString(imageID) {
		return observation
	}
	observation["status"] = "observed"
	observation["runtimeImageId"] = imageID

	output, err = exec.CommandContext(ctx, "docker", "image", "inspect", "--format", "{{json .}}", imageID).Output()
	var image struct {
		OS, Architecture, Variant string
		RepoDigests               []string
	}
	if err != nil || json.Unmarshal(output, &image) != nil {
		return observation
	}
	if image.OS != "" && image.Architecture != "" {
		platform := image.OS + "/" + image.Architecture
		if image.Variant != "" {
			platform += "/" + image.Variant
		}
		observation["platform"] = platform
	}
	for _, reference := range image.RepoDigests {
		if reference != requested {
			continue
		}
		if _, digest, ok := strings.Cut(reference, "@"); ok && dockerImageID.MatchString(digest) {
			observation["registryDigest"] = digest
		}
	}
	return observation
}
