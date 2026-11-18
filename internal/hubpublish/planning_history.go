package hubpublish

import (
	"context"
	"fmt"
	"strings"
	"time"

	apirequests "github.com/UFFeScience/akoflow/internal/api/requests"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/google/go-containerregistry/pkg/name"
)

func (m *Manager) HistoricalProfiles(ctx context.Context, workflow domain.WorkflowDefinition, resources []domain.Resource, environments []domain.EnvironmentDefinition) []domain.ActivityResourceProfile {
	m.mu.Lock()
	source := m.state.WorkflowSources[workflow.Version.ID]
	origin := m.state.HubURL
	m.mu.Unlock()
	if source.LocalHash == "" || source.HubOrigin != origin || source.PublicationID == "" || source.Version < 1 {
		return nil
	}
	yaml, err := apirequests.FromDomain(workflow).YAML()
	if err != nil || digest(yaml) != source.LocalHash {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var profiles []domain.ActivityResourceProfile
	type cacheKey struct {
		architecture, driver, model string
		cores                       int
		memory                      int64
	}
	byResourceShape := make(map[cacheKey]map[string]ActivityHistoryEstimate)
	for _, resource := range resources {
		if ctx.Err() != nil {
			break
		}
		architecture := historyArchitecture(resource.Architecture)
		driver := historyRuntime(resource.ID, environments)
		if architecture == "" || driver == "" {
			continue
		}
		model, _ := resource.Metadata["cpuModel"].(string)
		if !cpuModel.MatchString(model) {
			model = ""
		}
		key := cacheKey{architecture, driver, model, resource.CPUCores, resource.MemoryBytes}
		estimates, known := byResourceShape[key]
		if !known {
			request := historyProfilesForResource(workflow.Version, resource, architecture, driver, model)
			estimates, _ = m.activityEstimates(ctx, historySourceURI(source.PublicationID, source.Version), origin, request, source.Reference)
			byResourceShape[key] = estimates
		}
		for _, activity := range workflow.Version.Activities {
			estimate, found := estimates[historyActivityID(activity)]
			if !found {
				continue
			}
			metadata := map[string]any{"minSeconds": estimate.MinSeconds, "maxSeconds": estimate.MaxSeconds}
			if estimate.PreparationMedianSeconds != nil && *estimate.PreparationMedianSeconds >= 0 {
				metadata["preparationMedianSeconds"] = *estimate.PreparationMedianSeconds
			}
			profiles = append(profiles, domain.ActivityResourceProfile{
				ID:         "hub-history:" + activity.ID + ":" + resource.ID,
				ActivityID: activity.ID, ResourceID: resource.ID, RuntimeSeconds: estimate.MedianSeconds,
				Source: "hub-history", SampleSize: estimate.SampleCount,
				Metadata: metadata,
			})
		}
	}
	return profiles
}

func historyProfilesForResource(workflow domain.WorkflowVersion, resource domain.Resource, architecture, driver, model string) map[string]ActivityHistoryProfile {
	request := make(map[string]ActivityHistoryProfile)
	for _, activity := range workflow.Activities {
		if !activity.Supports(domain.ActivityCapabilityReal) {
			continue
		}
		image := activity.Command.Image
		if executable := activity.Command.EffectiveExecutable(); executable != nil {
			if executable.Source.Type != domain.ExecutableSourceOCI {
				continue
			}
			image = executable.Source.Reference
		}
		runtime := driver
		if driver == "local" {
			runtime = "Native process"
			if image != "" {
				runtime = "Docker"
			}
		}
		profile := ActivityHistoryProfile{
			Architecture: architecture, Runtime: runtime, CPUModel: model,
			CPUCores: float64(resource.CPUCores), MemoryBytes: float64(resource.MemoryBytes),
			AllocatedCPU: activity.Resources.CPU, AllocatedMemory: float64(activity.Resources.MemoryBytes),
			InputBytes: declaredInputBytes(workflow, activity),
		}
		if pinned, err := name.NewDigest(strings.TrimPrefix(image, "docker://")); err == nil {
			profile.ImageDigest = pinned.DigestStr()
		}
		request[historyActivityID(activity)] = profile
	}
	return request
}

func historyActivityID(activity domain.Activity) string {
	if activity.ExternalID != "" {
		return activity.ExternalID
	}
	return activity.ID
}

func historyRuntime(resourceID string, environments []domain.EnvironmentDefinition) string {
	var driver string
	for _, environment := range environments {
		for _, binding := range environment.RuntimeBindings {
			if !binding.Enabled || binding.ResourceID != resourceID {
				continue
			}
			for _, runtime := range environment.Runtimes {
				if runtime.ID != binding.RuntimeID || runtime.Mode != domain.RuntimeModeExecution {
					continue
				}
				name := ""
				switch runtime.Driver {
				case domain.RuntimeDriverSlurm:
					name = "Slurm"
				case domain.RuntimeDriverLocal:
					name = "local"
				}
				if name == "" || driver != "" && driver != name {
					return ""
				}
				driver = name
			}
		}
	}
	return driver
}

func historyArchitecture(architecture string) string {
	switch strings.ToLower(architecture) {
	case "amd64", "x86_64":
		return "amd64"
	case "arm64", "aarch64":
		return "arm64"
	case "ppc64le", "s390x", "riscv64":
		return strings.ToLower(architecture)
	default:
		return ""
	}
}

func historySourceURI(publicationID string, version int) string {
	return fmt.Sprintf("akoflow://hub/publications/%s/versions/%d", publicationID, version)
}

func declaredInputBytes(workflow domain.WorkflowVersion, activity domain.Activity) int64 {
	if seed, _ := activity.Metadata["workspaceSeedPath"].(string); seed != "" {
		return 0
	}
	var total int64
	const maxBytes = 1 << 53
	for _, input := range workflow.DataDependencies {
		if input.ConsumerActivityID != activity.ID {
			continue
		}
		if input.SizeBytes <= 0 || input.SizeBytes > maxBytes-total {
			return 0
		}
		total += input.SizeBytes
	}
	return total
}
