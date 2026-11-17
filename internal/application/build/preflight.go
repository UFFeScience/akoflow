package build

import (
	"context"
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/operationerror"
)

func (m Manager) CheckExecutable(ctx context.Context, source domain.ExecutableSource) (bool, error) {
	if source.Origin != nil {
		if err := source.Origin.Validate(); err != nil {
			return true, err
		}
		if source.Origin.Build != nil {
			if err := inspectPortableContext(*source.Origin.Build); err != nil {
				return true, err
			}
			if m.Executor.Runner == nil {
				return false, nil
			}
			name := m.Executor.Buildctl
			if name == "" {
				name = "buildctl"
			}
			_, err := m.Executor.Runner.Run(ctx, name, []string{"debug", "workers"}, nil)
			if err != nil {
				return true, operationerror.Wrap(err, "builder_unavailable", "artifact_preparation", "", true, "Restore the configured BuildKit service.")
			}
			return false, nil
		}
	}
	image := source.Reference
	if source.Origin != nil {
		image = source.Origin.Image
	}
	if image == "" || m.Executor.Runner == nil {
		return false, nil
	}
	if strings.HasPrefix(image, "-") || strings.ContainsAny(image, " \r\n\t") {
		return true, fmt.Errorf("invalid OCI image reference")
	}
	if source.Origin != nil {
		reproducer := &Reproducer{Runner: m.Executor.Runner}
		if _, err := reproducer.inspect(ctx, image, *source.Origin); err == nil {
			return true, nil
		}
	} else if _, err := m.Executor.Runner.Run(ctx, "docker", []string{"image", "inspect", image}, nil); err == nil {
		return true, nil
	}
	_, err := m.Executor.Runner.Run(ctx, "docker", []string{"manifest", "inspect", image}, nil)
	if err != nil {
		return true, operationerror.Wrap(err, "registry_access_failed", "artifacts", "", true, "Check the image reference, registry credentials and network access.")
	}
	return true, nil
}
