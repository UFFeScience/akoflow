package workflow_engine_api_handler

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
)

func pinInspectedOCI(source *domain.ExecutableSource) error {
	if source.Inspection == nil || source.Origin != nil || (source.Type != domain.ExecutableSourceOCI && source.Type != domainworkflow.ExecutableSourceLocalContainerImage) {
		return nil
	}
	data := source.Inspection.Data
	digest, _ := data["Id"].(string)
	if digest == "" {
		return nil
	}
	reference := source.Reference
	if !strings.Contains(reference, "@sha256:") {
		encoded, _ := json.Marshal(data["RepoDigests"])
		var references []string
		_ = json.Unmarshal(encoded, &references)
		reference = ""
		for _, candidate := range references {
			if inspectionRepository(candidate) == inspectionRepository(source.Reference) {
				reference = candidate
				break
			}
		}
	}
	if reference == "" {
		return fmt.Errorf("publish this local image to a registry or attach its Dockerfile context before exporting")
	}
	if source.Type == domain.ExecutableSourceOCI && source.ExpectedDigest == "" {
		encoded, _ := json.Marshal(data["platforms"])
		var platforms []string
		if json.Unmarshal(encoded, &platforms) == nil && len(platforms) > 1 {
			source.Reference = reference
			source.Inspection.Data["reference"] = reference
			return nil
		}
	}
	platform, _ := data["platform"].(string)
	origin := &domain.ArtifactOrigin{Image: reference, Platform: platform, ExpectedDigest: digest}
	if err := origin.Validate(); err != nil {
		return err
	}
	source.Type = domain.ExecutableSourceOCI
	source.Reference = reference
	source.Origin = origin
	source.ExpectedDigest = digest
	source.Inspection.Data["reference"] = reference
	return nil
}

func inspectionRepository(reference string) string {
	reference = strings.SplitN(strings.TrimPrefix(reference, "docker://"), "@", 2)[0]
	if colon := strings.LastIndex(reference, ":"); colon > strings.LastIndex(reference, "/") {
		reference = reference[:colon]
	}
	reference = strings.TrimPrefix(reference, "docker.io/")
	reference = strings.TrimPrefix(reference, "index.docker.io/")
	if !strings.Contains(reference, "/") {
		reference = "library/" + reference
	}
	return reference
}
