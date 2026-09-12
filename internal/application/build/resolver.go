package build

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// OutputResolver converts a completed immutable build into the same verified
// materialization contract used by catalog artifacts.
type OutputResolver struct {
	Catalog interface {
		FindBuildOutput(context.Context, string) (*domain.ArtifactVariant, *domain.ArtifactLocation, error)
		FindDockerBuildOutput(context.Context, string, string) (*domain.ArtifactBuild, *domain.ArtifactVariant, *domain.ArtifactLocation, error)
	}
}

type CatalogOutput interface {
	FindCatalogOutput(context.Context, string, string, string) (*domain.ArtifactVariant, *domain.ArtifactLocation, error)
}

type CatalogOCIReference interface {
	FindCatalogOCIReference(context.Context, string, string, string) (string, error)
}

func OCIReferenceForCatalog(ctx context.Context, catalog CatalogOCIReference, artifactID, version, architecture string) (string, error) {
	if catalog == nil {
		return "", fmt.Errorf("artifact catalog OCI resolver is unavailable")
	}
	reference, err := catalog.FindCatalogOCIReference(ctx, artifactID, version, normalizedArchitecture(architecture))
	if err != nil {
		return "", err
	}
	if reference == "" {
		return "", fmt.Errorf("catalog artifact %q version %q has no OCI source for Kubernetes", artifactID, version)
	}
	return reference, nil
}

func PreparationForCatalog(ctx context.Context, catalog CatalogOutput, artifactID, version, activityID string, resource domain.Resource, destination string) (domain.PreparationRequirement, error) {
	if catalog == nil {
		return domain.PreparationRequirement{}, fmt.Errorf("artifact catalog is unavailable")
	}
	variant, location, err := catalog.FindCatalogOutput(ctx, artifactID, version, normalizedArchitecture(resource.Architecture))
	if err != nil {
		return domain.PreparationRequirement{}, err
	}
	if variant == nil || location == nil {
		return domain.PreparationRequirement{}, fmt.Errorf("catalog artifact %q version %q has no available variant for %q", artifactID, version, normalizedArchitecture(resource.Architecture))
	}
	return preparationForOutput(variant, location, activityID, resource, destination), nil
}

func (r OutputResolver) Preparation(ctx context.Context, buildID, activityID string, resource domain.Resource, destination string) (domain.PreparationRequirement, error) {
	if r.Catalog == nil {
		return domain.PreparationRequirement{}, fmt.Errorf("build output catalog is unavailable")
	}
	variant, location, err := r.Catalog.FindBuildOutput(ctx, buildID)
	if err != nil {
		return domain.PreparationRequirement{}, err
	}
	if variant == nil || location == nil {
		return domain.PreparationRequirement{}, fmt.Errorf("build %q has no completed output", buildID)
	}
	return preparationForOutput(variant, location, activityID, resource, destination), nil
}

// PreparationForDockerImage resolves a completed Docker-backed SIF build. It
// lets legacy OCI workflow definitions run on Slurm once the matching image
// has been materialized through the Artifact catalog.
func (r OutputResolver) PreparationForDockerImage(ctx context.Context, image, activityID string, resource domain.Resource, destination string) (domain.PreparationRequirement, bool, error) {
	if r.Catalog == nil {
		return domain.PreparationRequirement{}, false, fmt.Errorf("build output catalog is unavailable")
	}
	_, variant, location, err := r.Catalog.FindDockerBuildOutput(ctx, image, normalizedArchitecture(resource.Architecture))
	if err != nil {
		return domain.PreparationRequirement{}, false, err
	}
	if variant == nil || location == nil {
		return domain.PreparationRequirement{}, false, nil
	}
	return preparationForOutput(variant, location, activityID, resource, destination), true, nil
}

func normalizedArchitecture(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "x86_64", "x64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return value
	}
}

func preparationForOutput(variant *domain.ArtifactVariant, location *domain.ArtifactLocation, activityID string, resource domain.Resource, destination string) domain.PreparationRequirement {
	destinationURI := ""
	materializedPath := destination
	if destination == "" {
		workingDirectory, err := filepath.Abs(".")
		if err != nil {
			workingDirectory = "."
		}
		destination = filepath.Join(".akoflow", "artifacts")
		destinationURI = (&url.URL{Scheme: "file", Path: workingDirectory}).String()
		materializedPath = filepath.Join(workingDirectory, destination, variant.Digest)
	} else {
		// Explicit destinations are replaced by the SSH/Kubernetes endpoint
		// configuration before transfer. Keep a syntactically valid local URL
		// as the fallback instead of allowing a relative path to become a host.
		destinationURI = (&url.URL{Scheme: "file", Path: "/"}).String()
		destination = strings.TrimPrefix(filepath.Clean(destination), string(filepath.Separator))
	}
	materialization := domain.ArtifactMaterialization{
		ID:         "materialization-" + activityID + "-" + variant.ID,
		ActivityID: activityID, VariantID: variant.ID, Digest: variant.Digest,
		ResourceID: resource.ID, EnvironmentID: resource.EnvironmentVersionID,
		DestinationPath: materializedPath, Status: domain.MaterializationPlanned,
	}
	transfer := domain.DataTransferPlan{
		ID: "transfer-" + materialization.ID, Strategy: domain.TransferSourcePush,
		Source: domain.TransferLocation{URI: location.URI},
		Destination: domain.TransferLocation{
			ResourceID: resource.ID, EnvironmentID: resource.EnvironmentVersionID,
			Path: destination, URI: destinationURI,
		},
		Blobs: []domain.BlobDescriptor{{Digest: variant.Digest, SizeBytes: variant.SizeBytes}},
	}
	return domain.PreparationRequirement{Artifact: &materialization, ArtifactTransfer: &transfer}
}
