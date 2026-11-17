package provider

import (
	"context"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// ResolveOCIPlatforms reads the index or the image configuration without downloading layers.
func ResolveOCIPlatforms(ctx context.Context, image string) (platforms []string, reference string) {
	ref, err := name.ParseReference(strings.TrimSpace(image))
	if err != nil {
		return nil, ""
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	descriptor, err := remote.Get(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, ""
	}
	reference = ref.Context().Name() + "@" + descriptor.Digest.String()
	if descriptor.MediaType.IsIndex() {
		index, err := descriptor.ImageIndex()
		if err != nil {
			return nil, ""
		}
		manifest, err := index.IndexManifest()
		if err != nil {
			return nil, ""
		}
		supported := make([]registryPlatform, 0, len(manifest.Manifests))
		for _, entry := range manifest.Manifests {
			if entry.Platform != nil {
				supported = append(supported, registryPlatform{OS: entry.Platform.OS, Architecture: entry.Platform.Architecture})
			}
		}
		return supportedRegistryPlatforms(supported), reference
	}
	remoteImage, err := descriptor.Image()
	if err != nil {
		return nil, ""
	}
	config, err := remoteImage.ConfigFile()
	if err != nil {
		return nil, ""
	}
	return supportedRegistryPlatforms([]registryPlatform{{OS: config.OS, Architecture: config.Architecture}}), reference
}
