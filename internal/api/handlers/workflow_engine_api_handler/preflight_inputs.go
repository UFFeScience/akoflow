package workflow_engine_api_handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	apppreflight "github.com/UFFeScience/akoflow/internal/application/preflight"
	domainpreflight "github.com/UFFeScience/akoflow/internal/domain/preflight"
)

func (h *Handler) preflightInputs(request ports.ExecutionRequest) []apppreflight.Probe {
	probes := []apppreflight.Probe{}
	for _, activity := range request.Workflow.Activities {
		probes = append(probes, apppreflight.Probe{Code: "required_inputs", Stage: "inputs", EntityID: activity.ID, Run: func(ctx context.Context) (string, string, string) {
			raw, declared := activity.Metadata["requiredInputs"]
			if !declared {
				return "unverified", "External file requirements are not declared; command text is not used to infer inputs.", "Declare requiredInputs in activity metadata to validate existing storage files."
			}
			payload, err := json.Marshal(raw)
			if err != nil {
				return "blocked", "Invalid requiredInputs declaration.", "Correct the workflow input declarations."
			}
			var inputs []domainpreflight.Input
			if err := json.Unmarshal(payload, &inputs); err != nil {
				return "blocked", "requiredInputs must be a list of storageId/path references.", "Correct the workflow input declarations."
			}
			for _, input := range inputs {
				if input.ProducerActivityID != "" {
					dependency := false
					for _, edge := range request.Workflow.Dependencies {
						dependency = dependency || edge.ActivityID == activity.ID && edge.DependsOnActivityID == input.ProducerActivityID
					}
					for _, edge := range request.Workflow.DataDependencies {
						dependency = dependency || edge.ConsumerActivityID == activity.ID && edge.ProducerActivityID == input.ProducerActivityID
					}
					if request.ActivityTest != nil {
						for _, recorded := range request.ActivityTest.Inputs[activity.ID] {
							dependency = dependency || recorded.ProducerActivityID == input.ProducerActivityID
						}
					}
					if !dependency {
						return "blocked", "Input producer is not a declared predecessor of this activity.", "Declare the producing activity dependency."
					}
					continue
				}
				if input.StorageID == "" || input.Path == "" {
					return "blocked", "External inputs require storageId and path.", "Complete the input storage reference."
				}
				if h.storage == nil {
					return "blocked", "Required input storage cannot be inspected.", "Configure the storage browser for the required inputs."
				}
				if _, err := h.storage.Stat(ctx, input.StorageID, input.Path); err != nil {
					return "blocked", fmt.Sprintf("Required input %q is unavailable: %v", input.Path, err), "Restore the file or correct its storage reference."
				}
			}
			return "passed", "Declared existing inputs are accessible; predecessor outputs are checked when produced.", ""
		}})
	}
	return probes
}
