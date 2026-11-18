package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/UFFeScience/akoflow/internal/application/imagepreparation"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func (s Executor) buildPinnedSIF(ctx context.Context, image, architecture, apptainer, sifPath string) (string, error) {
	workspace, err := os.MkdirTemp(filepath.Dir(sifPath), "sif-image-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(workspace)
	archive := filepath.Join(workspace, "image.tar")
	if err := imagepreparation.Observe(ctx, domainexecution.ImagePreparationPull, func() error {
		ref, err := name.NewDigest(image)
		if err != nil {
			return err
		}
		platform := v1.Platform{OS: "linux", Architecture: architecture}
		payload, err := remote.Image(ref, remote.WithContext(ctx), remote.WithPlatform(platform), remote.WithAuthFromKeychain(authn.DefaultKeychain))
		if err != nil {
			return err
		}
		config, err := payload.ConfigFile()
		if err != nil {
			return err
		}
		if config.OS != platform.OS || config.Architecture != platform.Architecture {
			return fmt.Errorf("OCI image platform is %s/%s, expected %s", config.OS, config.Architecture, platform.String())
		}
		return tarball.WriteToFile(archive, nil, payload)
	}); err != nil {
		return "", fmt.Errorf("pull pinned image: %w", err)
	}
	var output []byte
	if err := imagepreparation.Observe(ctx, domainexecution.ImagePreparationSIF, func() error {
		var err error
		output, err = s.Runner.Run(ctx, apptainer, []string{"build", sifPath, "docker-archive:" + archive}, nil)
		return err
	}); err != nil {
		return string(output), fmt.Errorf("build SIF: %w", err)
	}
	return string(output), nil
}
