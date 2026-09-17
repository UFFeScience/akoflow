package transfer

import (
	"context"
	"fmt"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type MaterializationCatalog interface {
	SaveArtifactMaterialization(context.Context, domain.ArtifactMaterialization) error
	SaveTransferRun(context.Context, domain.DataTransferRun) error
}

// Coordinator is the orchestration boundary: it returns only verified,
// committed materializations to providers.
type Coordinator struct {
	Materializer    Materializer
	Catalog         MaterializationCatalog
	WorkspaceSyncer WorkspaceSyncer
}

// WorkspaceSyncer synchronizes complete predecessor snapshots and rejects
// conflicting paths before making any successor workspace available.
type WorkspaceSyncer interface {
	Sync(context.Context, []domain.DataTransferPlan) ([]domain.DataTransferRun, error)
}

func (c Coordinator) Prepare(ctx context.Context, activityID string, requirement domain.PreparationRequirement) (*domain.PreparationGate, error) {
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
		plan := *requirement.ArtifactTransfer
		plan.ExecutionRunID = requirement.Artifact.RunID
		plan.ConsumerActivityID = activityID
		if err := c.saveTransfer(ctx, domain.DataTransferRun{
			ID: plan.ID, PlanID: plan.ID, ExecutionRunID: plan.ExecutionRunID, ActivityID: activityID,
			Strategy: requirement.ArtifactTransfer.Strategy, Status: domain.TransferRunning,
			StartedAt: float64(time.Now().UnixNano()) / float64(time.Second),
		}); err != nil {
			return nil, fmt.Errorf("start artifact transfer log: %w", err)
		}
		result, transferRun, err := c.Materializer.Materialize(ctx, plan, initial)
		if saveErr := c.saveTransfer(context.WithoutCancel(ctx), transferRun); saveErr != nil {
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
		runs, err := c.prepareWorkspace(ctx, activityID, &requirement)
		if err != nil {
			return nil, err
		}
		transferRuns = append(transferRuns, runs...)
	}
	gate := &domain.PreparationGate{Executable: requirement.Artifact, Workspace: requirement.Workspace, TransferRuns: transferRuns}
	if err := gate.Ready(); err != nil {
		return nil, err
	}
	return gate, nil
}

func (c Coordinator) prepareWorkspace(ctx context.Context, activityID string, requirement *domain.PreparationRequirement) ([]domain.DataTransferRun, error) {
	plans := append([]domain.DataTransferPlan(nil), requirement.WorkspaceTransfers...)
	if requirement.WorkspaceTransfer != nil {
		plans = append(plans, *requirement.WorkspaceTransfer)
	}
	if len(plans) == 0 && len(requirement.Workspace.Missing) > 0 {
		return nil, fmt.Errorf("workspace materialization lacks transfer plan")
	}
	if len(plans) > 0 && plans[0].SyncWorkspace {
		return c.prepareSnapshotWorkspace(ctx, activityID, requirement.Workspace, plans)
	}
	return c.prepareBlobWorkspace(ctx, activityID, requirement.Workspace, plans)
}

func (c Coordinator) prepareSnapshotWorkspace(ctx context.Context, activityID string, workspace *domain.WorkspaceMaterialization, plans []domain.DataTransferPlan) ([]domain.DataTransferRun, error) {
	if c.WorkspaceSyncer == nil {
		return nil, fmt.Errorf("workspace rsync is not configured; successor remains blocked")
	}
	for _, plan := range plans {
		if !plan.SyncWorkspace {
			return nil, fmt.Errorf("cannot mix snapshot and blob workspace transfers")
		}
		if err := c.saveTransfer(ctx, domain.DataTransferRun{
			ID: plan.ID, PlanID: plan.ID, ExecutionRunID: plan.ExecutionRunID,
			ActivityID: activityID, Status: domain.TransferRunning,
			StartedAt: float64(time.Now().UnixNano()) / float64(time.Second),
		}); err != nil {
			return nil, err
		}
	}
	runs, syncErr := c.WorkspaceSyncer.Sync(ctx, plans)
	for _, run := range runs {
		if err := c.saveTransfer(context.WithoutCancel(ctx), run); err != nil {
			return nil, err
		}
	}
	if syncErr != nil {
		return nil, fmt.Errorf("workspace synchronization failed (retry the run after correcting the transfer): %w", syncErr)
	}
	if len(runs) != len(plans) {
		return nil, fmt.Errorf("workspace synchronization returned %d of %d dependency transfers", len(runs), len(plans))
	}
	for i, run := range runs {
		if run.ID != plans[i].ID || run.Status != domain.TransferCompleted {
			return nil, fmt.Errorf("workspace dependency transfer %q was not completed", plans[i].ID)
		}
	}
	if err := workspace.Commit(nil); err != nil {
		return nil, err
	}
	return runs, nil
}

func (c Coordinator) prepareBlobWorkspace(ctx context.Context, activityID string, workspace *domain.WorkspaceMaterialization, plans []domain.DataTransferPlan) ([]domain.DataTransferRun, error) {
	if len(workspace.Missing) == 0 {
		for _, plan := range plans {
			workspace.Missing = append(workspace.Missing, plan.Blobs...)
		}
	}
	verified := make([]string, 0, len(workspace.Missing))
	runs := make([]domain.DataTransferRun, 0, len(plans))
	for _, originalPlan := range plans {
		plan := originalPlan
		if plan.ExecutionRunID == "" {
			plan.ExecutionRunID = workspace.RevisionID
		}
		if plan.ConsumerActivityID == "" {
			plan.ConsumerActivityID = activityID
		}
		if err := c.saveTransfer(ctx, domain.DataTransferRun{
			ID: plan.ID, PlanID: plan.ID, ExecutionRunID: plan.ExecutionRunID,
			ActivityID: plan.ConsumerActivityID, Strategy: plan.Strategy,
			Status: domain.TransferRunning, StartedAt: float64(time.Now().UnixNano()) / float64(time.Second),
		}); err != nil {
			return nil, fmt.Errorf("start workspace transfer log: %w", err)
		}
		result, run, err := c.Materializer.Materialize(ctx, plan, domain.ArtifactMaterialization{ID: workspace.ID, Digest: "workspace"})
		if saveErr := c.saveTransfer(context.WithoutCancel(ctx), run); saveErr != nil {
			return nil, fmt.Errorf("save workspace transfer: %w", saveErr)
		}
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
		if result.Status != domain.MaterializationCommitted {
			return nil, fmt.Errorf("workspace materialization is not committed")
		}
		verified = append(verified, run.VerifiedBlobs...)
	}
	if err := workspace.Commit(verified); err != nil {
		return nil, err
	}
	return runs, nil
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
