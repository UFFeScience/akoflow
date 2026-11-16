package workflow_engine_api_handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/infrastructure/dockerhub"
)

type catalogBuildOrigin interface {
	FindArtifactBuildForCatalog(context.Context, string, string) (*domain.ArtifactBuild, error)
}

func (h *Handler) artifactOrigin(ctx context.Context, source domain.ExecutableSource) (*domain.ArtifactOrigin, error) {
	if source.Origin != nil {
		return originWithExpectedDigest(source, source.Origin)
	}
	if h.data == nil {
		return nil, nil
	}
	var spec *domain.ArtifactBuild
	var err error
	switch source.Type {
	case domain.ExecutableSourceBuild:
		spec, err = h.data.FindArtifactBuild(ctx, source.ArtifactBuildRef)
	case domain.ExecutableSourceCatalog:
		resolver, ok := h.data.(catalogBuildOrigin)
		if !ok || source.ArtifactRef == nil {
			return nil, nil
		}
		spec, err = resolver.FindArtifactBuildForCatalog(ctx, source.ArtifactRef.ID, source.ArtifactRef.Version)
	default:
		return nil, nil
	}
	if err != nil || spec == nil {
		return nil, err
	}
	exporter, ok := h.build.(interface {
		PortableOrigin(context.Context, domain.ArtifactBuild) (*domain.ArtifactOrigin, error)
	})
	if !ok {
		return nil, fmt.Errorf("portable artifact export is unavailable")
	}
	origin, err := exporter.PortableOrigin(ctx, *spec)
	if err != nil {
		return nil, err
	}
	if origin.Image != "" && !strings.Contains(origin.Image, "@") {
		if h.dockerHub == nil {
			return nil, fmt.Errorf("pin the image to a registry SHA-256 digest before exporting")
		}
		ref := strings.TrimPrefix(origin.Image, "docker.io/")
		split := strings.LastIndex(ref, ":")
		tag := "latest"
		if split > strings.LastIndex(ref, "/") {
			tag = ref[split+1:]
			ref = ref[:split]
		}
		image, err := h.dockerHub.Resolve(ctx, dockerhub.Selection{Repository: ref, Tag: tag}, spec.TargetArchitecture)
		if err != nil {
			return nil, fmt.Errorf("pin artifact OCI origin: %w", err)
		}
		origin.Image = image
	}
	return originWithExpectedDigest(source, origin)
}

func originWithExpectedDigest(source domain.ExecutableSource, origin *domain.ArtifactOrigin) (*domain.ArtifactOrigin, error) {
	copy := *origin
	if source.ExpectedDigest != "" {
		if copy.ExpectedDigest != "" && copy.ExpectedDigest != source.ExpectedDigest {
			return nil, fmt.Errorf("source and origin expectedDigest constraints disagree")
		}
		copy.ExpectedDigest = source.ExpectedDigest
	}
	return &copy, copy.Validate()
}

func (h *Handler) portableDefinition(ctx context.Context, definition domain.WorkflowDefinition) (domain.WorkflowDefinition, error) {
	payload, err := json.Marshal(definition)
	if err != nil {
		return definition, err
	}
	var portable domain.WorkflowDefinition
	if err := json.Unmarshal(payload, &portable); err != nil {
		return definition, err
	}
	for i := range portable.Version.Activities {
		command := &portable.Version.Activities[i].Command
		if command.Executable == nil {
			continue
		}
		source := &command.Executable.Source
		if source.Type != domain.ExecutableSourceCatalog && source.Type != domain.ExecutableSourceBuild {
			continue
		}
		origin, err := h.artifactOrigin(ctx, *source)
		if err != nil {
			return definition, fmt.Errorf("activity %q artifact origin: %w", portable.Version.Activities[i].Name, err)
		}
		if origin == nil {
			return definition, fmt.Errorf("activity %q has only a local artifact identifier; register its image digest or upload its source build context before exporting", portable.Version.Activities[i].Name)
		}
		source.Origin = origin
		source.ArtifactRef = nil
		source.ArtifactBuildRef = ""
		if origin.Image != "" {
			source.Type = domain.ExecutableSourceOCI
			source.Reference = origin.Image
		}
	}
	return portable, nil
}

func normalizedOriginArchitecture(architecture string) string {
	switch architecture {
	case "x86_64", "x64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return architecture
	}
}

func (h *Handler) preparePortableOrigin(ctx context.Context, request *ports.ExecutionRequest, index int, assignment domain.PlanAssignment, resource domain.Resource) (bool, error) {
	if executionRuntimeDriver(*request, assignment) != domain.RuntimeDriverLocal {
		return false, nil
	}
	activity := &request.Workflow.Activities[index]
	origin, err := h.artifactOrigin(ctx, activity.Command.Executable.Source)
	if err != nil {
		return false, fmt.Errorf("activity %q: %w", activity.ID, err)
	}
	if origin == nil {
		return false, nil
	}
	architecture := strings.Split(origin.Platform, "/")[1]
	if resource.Architecture != "" && architecture != normalizedOriginArchitecture(resource.Architecture) {
		return false, fmt.Errorf("activity %q artifact platform %q does not match resource architecture %q", activity.ID, origin.Platform, resource.Architecture)
	}
	if request.PreparationRequirementsByActivity == nil {
		request.PreparationRequirementsByActivity = make(map[string]domain.PreparationRequirement)
	}
	request.PreparationRequirementsByActivity[activity.ID] = domain.PreparationRequirement{Reproduction: origin}
	// Copy the authored source before enriching execution-owned metadata.
	reference := *activity.Command.Executable
	reference.Source.Origin = origin
	activity.Command.Executable = &reference
	return true, nil
}
