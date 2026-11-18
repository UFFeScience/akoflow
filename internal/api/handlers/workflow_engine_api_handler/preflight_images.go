package workflow_engine_api_handler

import (
	"context"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	apppreflight "github.com/UFFeScience/akoflow/internal/application/preflight"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/operationerror"
)

func (h *Handler) preflightImages(request *ports.ExecutionRequest) []apppreflight.Probe {
	probes := []apppreflight.Probe{}
	for index := range request.Workflow.Activities {
		activity := request.Workflow.Activities[index]
		probes = append(probes, apppreflight.Probe{Code: "executable_availability", Stage: "artifacts", EntityID: activity.ID, Run: func(ctx context.Context) (string, string, string) {
			if request.PreparationRequirementsByActivity[activity.ID].SIFImage != "" {
				return "unverified", "AkôFlow will resolve, reuse or build a compatible SIF before submission.", "Review the activity preparation steps if the image cannot be built."
			}
			if requirement, exists := request.PreparationRequirementsByActivity[activity.ID]; exists && requirement.Artifact != nil && requirement.Reproduction == nil {
				return "unverified",
					"A compatible catalog artifact is selected; its bytes and transfer route are verified during preparation.",
					"Review the artifact location and destination access. Registry access is not required to reuse this artifact."
			}
			source := domain.ExecutableSource{Type: domain.ExecutableSourceOCI, Reference: activity.Command.Image}
			if executable := request.Workflow.Activities[index].Command.Executable; executable != nil {
				source = executable.Source
			}
			if source.Type != domain.ExecutableSourceOCI && source.Origin == nil {
				return "unverified", "Executable catalog compatibility is checked separately; destination file access is verified during preparation.", "Review the selected artifact and its destination transfer route."
			}
			if source.Reference == "" && source.Origin == nil {
				return "unverified", "No inspectable OCI image reference is declared.", "Review the activity executable configuration."
			}
			checker, ok := h.build.(interface {
				CheckExecutable(context.Context, domain.ExecutableSource) (bool, error)
			})
			if !ok {
				return "unverified", "Read-only executable availability checks are unavailable.", "Review artifact availability before execution."
			}
			observed, err := checker.CheckExecutable(ctx, source)
			if err != nil {
				return "blocked", err.Error(), operationerror.Describe(err).NextAction
			}
			if source.Origin != nil && source.Origin.Build != nil {
				if observed {
					return "passed", "The locally built OCI image is verified and will be reused.", ""
				}
				return "warning", "Build required: the Dockerfile context and local builder are available.", "AkôFlow will build and verify the missing OCI image before starting this activity."
			}
			if !observed {
				return "warning", "Source context is valid, but build success cannot be confirmed without preparing the artifact.", "Preparation will build and verify the artifact before starting its activity."
			}
			return "passed", "The daemon can inspect the image locally or read its registry manifest; destination access is checked at launch.", ""
		}})
	}
	return probes
}
