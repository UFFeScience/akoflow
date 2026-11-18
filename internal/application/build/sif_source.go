package build

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/imagepreparation"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

func PinSIFSource(ctx context.Context, image string, runner CommandRunner) (string, error) {
	if _, err := name.NewDigest(image); err == nil {
		return image, nil
	}
	tag, err := name.NewTag(image)
	if err != nil {
		return "", reportSIFSourceFailure(ctx, err)
	}
	options := []remote.Option{remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain)}
	descriptor, err := remote.Head(tag, options...)
	if err == nil {
		return tag.Context().Digest(descriptor.Digest.String()).Name(), nil
	}
	var registryError *transport.Error
	// Only publish an existing local tag to the explicitly named registry.
	registry, _, qualified := strings.Cut(image, "/")
	if runner == nil || !qualified || (!strings.ContainsAny(registry, ".:") && registry != "localhost") ||
		!errors.As(err, &registryError) || registryError.StatusCode != http.StatusNotFound {
		return "", reportSIFSourceFailure(ctx, err)
	}
	if _, inspectErr := runner.Run(ctx, "docker", []string{"image", "inspect", tag.Name()}, nil); inspectErr != nil {
		return "", reportSIFSourceFailure(ctx, err)
	}
	var pinned string
	err = imagepreparation.Observe(ctx, domainexecution.ImagePreparationPush, func() error {
		if _, err := runner.Run(ctx, "docker", []string{"push", tag.Name()}, nil); err != nil {
			return fmt.Errorf("push OCI image %q: %w", tag.Name(), err)
		}
		descriptor, err := remote.Head(tag, options...)
		if err != nil {
			return fmt.Errorf("resolve pushed OCI digest: %w", err)
		}
		pinned = tag.Context().Digest(descriptor.Digest.String()).Name()
		return nil
	})
	return pinned, err
}

func reportSIFSourceFailure(ctx context.Context, err error) error {
	return imagepreparation.Observe(ctx, domainexecution.ImagePreparationPull, func() error {
		return fmt.Errorf("resolve OCI image digest: %w", err)
	})
}
