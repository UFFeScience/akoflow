package workflow_engine_api_handler

import (
	"context"
	"fmt"
	"strings"

	applicationbuild "github.com/UFFeScience/akoflow/internal/application/build"
	"github.com/UFFeScience/akoflow/internal/application/imagepreparation"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
	"github.com/UFFeScience/akoflow/internal/provider"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/uuid"
)

func (h *Handler) PrepareSIF(ctx context.Context, image string, activity domain.Activity, resource domain.Resource) (domain.PreparationRequirement, error) {
	var requirement domain.PreparationRequirement
	image = strings.TrimPrefix(strings.TrimSpace(image), "docker://")
	pinned, err := applicationbuild.PinSIFSource(ctx, image, provider.OSCommandExecutor{})
	if err != nil {
		return requirement, err
	}
	image = pinned
	if resource.Architecture == "" {
		resource.Architecture = "amd64"
	}
	resolver := applicationbuild.OutputResolver{Catalog: h.data}
	requirement, found, err := resolver.PreparationForDockerImage(ctx, image, activity.ID, resource, "")
	if err != nil {
		return requirement, err
	}
	if found {
		err = imagepreparation.Observe(ctx, domainexecution.ImagePreparationCacheHit, func() error { return nil })
	} else {
		if err := h.buildSIF(ctx, image, normalizedOriginArchitecture(resource.Architecture)); err != nil {
			return requirement, err
		}
		requirement, found, err = resolver.PreparationForDockerImage(ctx, image, activity.ID, resource, "")
		if err == nil && !found {
			err = fmt.Errorf("find registered SIF for %q: build output is unavailable", image)
		}
	}
	if err != nil {
		return requirement, err
	}
	err = h.configureGatewayArtifactTransfer(ctx, &requirement, resource, activity)
	return requirement, err
}

func (h *Handler) buildSIF(ctx context.Context, image, architecture string) error {
	builder, ok := h.build.(interface {
		BuildSIF(context.Context, domain.ArtifactBuild) error
	})
	if !ok {
		return fmt.Errorf("SIF builder is unavailable")
	}
	ref, err := name.NewDigest(image)
	if err != nil {
		return fmt.Errorf("invalid pinned OCI image: %w", err)
	}
	id := uuid.NewString()
	version := domain.ArtifactVersion{
		ID: "artifact-version-" + id, ArtifactID: ref.Context().Name(),
		Version: ref.DigestStr() + "-" + architecture + "-" + id, Scope: domain.CatalogScope("system"),
	}
	spec := domain.ArtifactBuild{
		ID: "build-" + id, ArtifactVersionID: version.ID, SourceType: "docker-image",
		ContextDigest: ref.DigestStr(), RecipePath: image, RecipeDigest: ref.DigestStr(),
		TargetFormat: "sif", TargetOS: "linux", TargetArchitecture: architecture,
		BuildArguments: "{}", CacheKey: "automatic-sif:" + id,
	}
	if err := h.data.RegisterArtifactVersion(ctx, version); err != nil {
		return imagepreparation.Observe(ctx, domainexecution.ImagePreparationRegister, func() error {
			return fmt.Errorf("register SIF artifact version: %w", err)
		})
	}
	if err := h.data.SaveArtifactBuild(ctx, spec); err != nil {
		return imagepreparation.Observe(ctx, domainexecution.ImagePreparationRegister, func() error {
			return fmt.Errorf("register SIF build: %w", err)
		})
	}
	return builder.BuildSIF(ctx, spec)
}
