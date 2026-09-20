package execution

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func (s *Supervisor) workspaceStore() ports.WorkspaceStore {
	store, _ := s.executions.(ports.WorkspaceStore)
	return store
}

func (s *Supervisor) RecoverWorkspaces(ctx context.Context) error {
	store := s.workspaceStore()
	if store == nil || s.config.Workspaces == nil {
		return nil
	}
	workspaces, err := store.ListWorkspaceCandidates(ctx)
	if err != nil {
		return err
	}
	for _, workspace := range workspaces {
		if workspace.State == domain.WorkspaceReleasing || workspace.State == domain.WorkspaceReleasable {
			workspace.State = domain.WorkspaceSealed
			if err := store.SaveWorkspace(ctx, workspace); err != nil {
				return err
			}
		}
		if err := s.reconcileWorkspace(ctx, workspace.ID); err != nil {
			return fmt.Errorf("recover workspace %q: %w", workspace.ID, err)
		}
	}
	return nil
}

func (s *Supervisor) initializeWorkspaces(ctx context.Context, request ports.ExecutionRequest, activities map[string]domain.Activity, assignments map[string]domain.PlanAssignment, resources map[string]domain.Resource) error {
	store := s.workspaceStore()
	if store == nil {
		return nil
	}
	now := time.Now().UTC()
	for activityID := range activities {
		assignment, assigned := assignments[activityID]
		resource, available := resources[assignment.ResourceID]
		if !assigned || !available {
			continue
		}
		location, err := workspaceDestination(request, activityID, resource, 0)
		if err != nil {
			continue
		}
		consumers := workspaceConsumers(request.Workflow, activityID)
		isFinal := len(consumers) == 0
		retention := domain.WorkspaceIntermediate
		if isFinal {
			retention = domain.WorkspaceFinal
		}
		workspace := domain.ActivityWorkspace{
			ID:    "workspace-" + request.Run.ID + "-" + activityID,
			RunID: request.Run.ID, ActivityID: activityID,
			EnvironmentID: resource.EnvironmentVersionID, ResourceID: resource.ID,
			RuntimeID: selectRuntime(request, assignment), ConnectionID: runtimeConnectionID(request, activityID),
			URI: normalizedWorkspaceURI(location), ExecutionPath: workspacePath(location.URI),
			ObservationPath: workspacePath(location.URI), Driver: workspaceDriver(request, activityID),
			State: domain.WorkspacePlanned, Retention: retention, IsFinal: isFinal, CreatedAt: now,
		}
		if err := store.SaveWorkspace(ctx, workspace); err != nil {
			return err
		}
		for _, consumerID := range consumers {
			lease := domain.WorkspaceLease{
				ID:          fmt.Sprintf("workspace-lease-%s-%s-%s", request.Run.ID, activityID, consumerID),
				WorkspaceID: workspace.ID, RunID: request.Run.ID,
				ProducerActivityID: activityID, ConsumerActivityID: consumerID,
			}
			if err := store.SaveWorkspaceLease(ctx, lease); err != nil {
				return err
			}
		}
	}
	return nil
}

func workspaceConsumers(workflow domain.WorkflowVersion, activityID string) []string {
	consumers := make([]string, 0)
	seen := make(map[string]bool)
	for _, activity := range workflow.Activities {
		for _, producerID := range workspaceProducers(workflow, activity.ID) {
			if producerID == activityID && !seen[activity.ID] {
				consumers = append(consumers, activity.ID)
				seen[activity.ID] = true
			}
		}
	}
	return consumers
}

func workspaceDriver(request ports.ExecutionRequest, activityID string) string {
	switch runtimeDriver(request, activityID) {
	case domain.RuntimeDriverSlurm:
		return "ssh"
	case domain.RuntimeDriverCloud:
		return "cloud-ssh"
	case domain.RuntimeDriverKubernetes:
		return "kubernetes-pvc"
	case domain.RuntimeDriverLocal:
		return "local"
	default:
		return string(runtimeDriver(request, activityID))
	}
}

func normalizedWorkspaceURI(location domain.TransferLocation) string {
	parsed, err := url.Parse(location.URI)
	if err != nil || parsed.Scheme != "file" {
		return location.URI
	}
	connectionID := location.ConnectionID
	if connectionID == "" {
		connectionID = parsed.Query().Get("connectionId")
	}
	if connectionID == "" {
		return location.URI
	}
	parsed.Scheme, parsed.Host, parsed.RawQuery = "ssh", connectionID, ""
	return parsed.String()
}

func workspacePath(rawURI string) string {
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return rawURI
	}
	if parsed.Path != "" {
		return parsed.Path
	}
	return strings.TrimSpace(rawURI)
}

func (s *Supervisor) activateWorkspace(ctx context.Context, runID, activityID string) error {
	store := s.workspaceStore()
	if store == nil {
		return nil
	}
	workspace, err := store.FindWorkspace(ctx, "workspace-"+runID+"-"+activityID)
	if err != nil || workspace == nil {
		return err
	}
	if s.config.Workspaces != nil {
		if err := s.config.Workspaces.Ensure(ctx, *workspace); err != nil {
			workspace.State, workspace.LastError = domain.WorkspaceFailed, err.Error()
			_ = store.SaveWorkspace(context.WithoutCancel(ctx), *workspace)
			return err
		}
	}
	workspace.State = domain.WorkspaceActive
	return store.SaveWorkspace(ctx, *workspace)
}

func (s *Supervisor) releaseProducerWorkspaceLeases(ctx context.Context, runID, consumerID string) error {
	store := s.workspaceStore()
	if store == nil {
		return nil
	}
	leases, err := store.ListWorkspaceLeases(ctx, runID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, lease := range leases {
		if lease.ConsumerActivityID != consumerID || lease.Released {
			continue
		}
		lease.Released, lease.ReleasedAt = true, &now
		lease.ReleaseReason = "consumer workspace materialized"
		if err := store.SaveWorkspaceLease(ctx, lease); err != nil {
			return err
		}
		if err := s.reconcileWorkspace(ctx, lease.WorkspaceID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Supervisor) sealWorkspace(ctx context.Context, runID, activityID string, manifest *domain.ArtifactManifest) error {
	store := s.workspaceStore()
	if store == nil {
		return nil
	}
	workspace, err := store.FindWorkspace(ctx, "workspace-"+runID+"-"+activityID)
	if err != nil || workspace == nil {
		return err
	}
	now := time.Now().UTC()
	workspace.State, workspace.SealedAt = domain.WorkspaceSealed, &now
	if s.config.Workspaces != nil {
		usage, inspectErr := s.config.Workspaces.Inspect(ctx, *workspace)
		if inspectErr != nil {
			workspace.LastError = inspectErr.Error()
		} else {
			workspace.FileCount, workspace.SizeBytes = usage.FileCount, usage.SizeBytes
		}
	}
	if manifest != nil {
		workspace.FileCount = int64(manifest.Summary.FinalFiles)
		workspace.OutputBytes = manifest.Summary.OutputBytes
		for _, file := range manifest.Files {
			entry := domain.WorkspaceEntry{Path: file.Path, Digest: file.Checksum, SizeBytes: file.SizeBytes}
			if file.Change == domain.ArtifactCreated || file.Change == domain.ArtifactModified {
				workspace.Manifest.Outputs = append(workspace.Manifest.Outputs, entry)
			}
		}
	}
	if err := store.SaveWorkspace(ctx, *workspace); err != nil {
		return err
	}
	return s.reconcileWorkspace(ctx, workspace.ID)
}

func (s *Supervisor) reconcileWorkspace(ctx context.Context, workspaceID string) error {
	store := s.workspaceStore()
	if store == nil {
		return nil
	}
	workspace, err := store.FindWorkspace(ctx, workspaceID)
	if err != nil || workspace == nil || workspace.State != domain.WorkspaceSealed || workspace.Pinned || workspace.IsFinal {
		return err
	}
	leases, err := store.ListWorkspaceLeases(ctx, workspace.RunID)
	if err != nil {
		return err
	}
	for _, lease := range leases {
		if lease.WorkspaceID == workspace.ID && !lease.Released {
			return nil
		}
	}
	workspace.State = domain.WorkspaceReleasable
	workspace.ReleaseReason = "all downstream consumer workspaces were materialized"
	if err := store.SaveWorkspace(ctx, *workspace); err != nil || s.config.Workspaces == nil {
		return err
	}
	workspace.State = domain.WorkspaceReleasing
	if err := store.SaveWorkspace(ctx, *workspace); err != nil {
		return err
	}
	result, releaseErr := s.config.Workspaces.Release(ctx, *workspace)
	if releaseErr != nil {
		workspace.State, workspace.LastError = domain.WorkspaceFailed, releaseErr.Error()
		_ = store.SaveWorkspace(context.WithoutCancel(ctx), *workspace)
		return releaseErr
	}
	now := time.Now().UTC()
	workspace.State, workspace.ReleasedAt = domain.WorkspaceReleased, &now
	workspace.ReclaimedBytes = result.ReclaimedBytes
	return store.SaveWorkspace(context.WithoutCancel(ctx), *workspace)
}
