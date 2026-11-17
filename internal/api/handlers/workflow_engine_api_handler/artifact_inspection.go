package workflow_engine_api_handler

import (
	"context"
	"fmt"
	"net/url"

	applicationbuild "github.com/UFFeScience/akoflow/internal/application/build"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
)

type executableInspector interface {
	InspectOrigin(context.Context, domain.ArtifactOrigin) (map[string]any, error)
	InspectSource(context.Context, domain.ExecutableSource) (map[string]any, error)
}

func (h *Handler) inspectArtifactSource(ctx context.Context, source domain.ExecutableSource) (map[string]any, error) {
	inspector, ok := h.build.(executableInspector)
	if !ok {
		return nil, fmt.Errorf("artifact inspection is unavailable")
	}
	variant, location, err := h.inspectionSIFOutput(ctx, source)
	if err != nil {
		return nil, err
	}
	if variant != nil && location != nil && variant.Format == "sif" && source.Format != domainworkflow.ExecutableFormatOCI {
		uri, err := url.Parse(location.URI)
		if err != nil {
			return nil, fmt.Errorf("invalid SIF artifact location")
		}
		fileSource := domain.ExecutableSource{Type: domain.ExecutableSourceRemoteFile, Format: domain.ExecutableFormatSIF, URI: location.URI, ExpectedDigest: variant.Digest}
		if uri.Scheme == "file" && (uri.Host == "" || uri.Host == "localhost") {
			fileSource.Type = domainworkflow.ExecutableSourceLocalFile
			fileSource.Path = uri.Path
		}
		result, err := inspector.InspectSource(ctx, fileSource)
		if err != nil {
			return nil, err
		}
		result["variant"] = variant
		result["location"] = location
		return result, nil
	}
	if source.Format == domain.ExecutableFormatSIF || source.Type == domainworkflow.ExecutableSourceLocalFile || source.Type == domain.ExecutableSourceRemoteFile {
		return inspector.InspectSource(ctx, source)
	}
	origin, err := h.artifactOrigin(ctx, source)
	if err != nil {
		return nil, err
	}
	if origin != nil {
		return inspector.InspectOrigin(ctx, *origin)
	}
	if variant != nil && location != nil {
		source = domain.ExecutableSource{Type: domain.ExecutableSourceOCI, Reference: variant.Digest, Format: domainworkflow.ExecutableFormatOCI}
		result, err := inspector.InspectSource(ctx, source)
		if err != nil {
			return nil, err
		}
		result["variant"], result["location"] = variant, location
		return result, nil
	}
	return inspector.InspectSource(ctx, source)
}

func (h *Handler) inspectionSIFOutput(ctx context.Context, source domain.ExecutableSource) (*domain.ArtifactVariant, *domain.ArtifactLocation, error) {
	if h.data == nil {
		return nil, nil, nil
	}
	if source.Type == domain.ExecutableSourceCatalog && source.ArtifactRef != nil {
		if catalog, ok := h.data.(applicationbuild.CatalogOutput); ok {
			return catalog.FindCatalogOutput(ctx, source.ArtifactRef.ID, source.ArtifactRef.Version, "")
		}
	}
	if source.Type == domain.ExecutableSourceBuild && source.ArtifactBuildRef != "" {
		return h.data.FindBuildOutput(ctx, source.ArtifactBuildRef)
	}
	return nil, nil, nil
}
