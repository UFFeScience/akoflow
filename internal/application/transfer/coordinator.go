package transfer

import (
	"context"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type MaterializationCatalog interface {
	SaveArtifactMaterialization(context.Context, domain.ArtifactMaterialization) error
	SaveTransferRun(context.Context, domain.DataTransferRun) error
}

// Coordinator is the orchestration boundary: it returns only verified,
// committed materializations to providers.
type Coordinator struct {
	Materializer Materializer
	Catalog      MaterializationCatalog
}

func (c Coordinator) Prepare(ctx context.Context, _ string, requirement domain.PreparationRequirement) (*domain.PreparationGate, error) {
	transferRuns := make([]domain.DataTransferRun, 0, 2)
	if requirement.Artifact != nil {
		if requirement.ArtifactTransfer == nil {
			return nil, fmt.Errorf("artifact materialization lacks transfer plan")
		}
		initial := *requirement.Artifact
		initial.Status = domain.MaterializationTransferring
		if err := c.save(ctx, initial); err != nil {
			return nil, fmt.Errorf("save artifact materialization: %w", err)
		}
		result, transferRun, err := c.Materializer.Materialize(ctx, *requirement.ArtifactTransfer, initial)
		if saveErr := c.saveTransfer(ctx, transferRun); saveErr != nil {
			return nil, fmt.Errorf("save artifact transfer: %w", saveErr)
		}
		if saveErr := c.save(ctx, result); saveErr != nil {
			return nil, fmt.Errorf("save artifact materialization: %w", saveErr)
		}
		if err != nil {
			return nil, err
		}
		transferRuns = append(transferRuns, transferRun)
		requirement.Artifact = &result
	}
	if requirement.Workspace != nil {
		if requirement.WorkspaceTransfer == nil {
			return nil, fmt.Errorf("workspace materialization lacks transfer plan")
		}
		// Workspace uses the same verified content transport. Artifact fields are
		// a small adapter around the common materializer.
		// Do not use a sentinel digest as proof of a workspace: each content blob
		// in the plan must be verified before the workspace can be committed.
		if len(requirement.Workspace.Missing) == 0 {
			requirement.Workspace.Missing = append([]domain.BlobDescriptor(nil), requirement.WorkspaceTransfer.Blobs...)
		}
		result, run, err := c.Materializer.Materialize(ctx, *requirement.WorkspaceTransfer, domain.ArtifactMaterialization{ID: requirement.Workspace.ID, Digest: "workspace"})
		if err != nil {
			return nil, err
		}
		transferRuns = append(transferRuns, run)
		if result.Status != domain.MaterializationCommitted {
			return nil, fmt.Errorf("workspace materialization is not committed")
		}
		if err := requirement.Workspace.Commit(run.VerifiedBlobs); err != nil {
			return nil, err
		}
	}
	gate := &domain.PreparationGate{Executable: requirement.Artifact, Workspace: requirement.Workspace, TransferRuns: transferRuns}
	if err := gate.Ready(); err != nil {
		return nil, err
	}
	return gate, nil
}

func (c Coordinator) saveTransfer(ctx context.Context, value domain.DataTransferRun) error {
	if c.Catalog == nil || value.ID == "" {
		return nil
	}
	return c.Catalog.SaveTransferRun(ctx, value)
}

func (c Coordinator) save(ctx context.Context, value domain.ArtifactMaterialization) error {
	if c.Catalog == nil {
		return nil
	}
	return c.Catalog.SaveArtifactMaterialization(ctx, value)
}
