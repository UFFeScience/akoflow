package workflow_engine_api_handler

import (
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/operationerror"
)

func validatePortableRuntime(activity domain.Activity, resource domain.Resource, driver domain.RuntimeDriver) error {
	if activity.Command.Executable == nil || activity.Command.Executable.Source.Origin == nil {
		return nil
	}
	origin := activity.Command.Executable.Source.Origin
	fail := func(err error, code, action string) error {
		return operationerror.Wrap(fmt.Errorf("activity %q: %w", activity.ID, err), code, "artifact_compatibility", activity.ID, false, action)
	}
	if err := origin.Validate(); err != nil {
		return fail(err, "artifact_origin_invalid", "Export this workflow again with a valid immutable artifact origin.")
	}
	architecture := strings.Split(origin.Platform, "/")[1]
	if (driver != domain.RuntimeDriverLocal || origin.Build != nil) && resource.Architecture != "" && architecture != normalizedOriginArchitecture(resource.Architecture) {
		return fail(
			fmt.Errorf("artifact platform %q does not match resource architecture %q", origin.Platform, resource.Architecture),
			"artifact_platform_mismatch",
			"Select a compatible resource or publish an image for the destination architecture.",
		)
	}
	if origin.Build != nil && driver != domain.RuntimeDriverLocal {
		return fail(
			fmt.Errorf("portable source builds cannot be prepared directly by runtime %q", driver),
			"artifact_runtime_unsupported",
			"Prepare the image locally, then publish an OCI image for Kubernetes or register a completed SIF build for Slurm.",
		)
	}
	return nil
}
