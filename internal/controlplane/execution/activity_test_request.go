package execution

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func ProjectActivityTestRequest(original ports.ExecutionRequest, runID string, graph ActivityTestGraph) (ports.ExecutionRequest, error) {
	if runID == "" || runID == original.Run.ID || len(graph.RunActivityIDs) == 0 {
		return ports.ExecutionRequest{}, fmt.Errorf("an activity test requires a separate run and at least one activity")
	}
	included := make(map[string]bool, len(graph.RunActivityIDs))
	for _, id := range graph.RunActivityIDs {
		included[id] = true
	}

	request := original
	request.Run = domain.ExecutionRun{
		ID: runID, Mode: domain.ExecutionModeReal, Seed: original.Run.Seed,
		Status: domain.ExecutionRunCreated, SchedulePlanID: original.Plan.ID,
		ActivityCount: len(included),
	}
	request.PreflightID = ""
	request.AcknowledgeWarnings = false
	request.Recovery = nil
	request.ActivityTest = nil
	request.RuntimeAllocations = nil
	request.PreparationRequirementsByActivity = nil
	request.Workflow.Activities = nil
	request.Workflow.Dependencies = nil
	request.Workflow.DataDependencies = nil
	request.Plan.Assignments = nil
	request.Plan.LifecycleActions = nil

	for _, activity := range original.Workflow.Activities {
		if included[activity.ID] {
			request.Workflow.Activities = append(request.Workflow.Activities, activity)
		}
	}
	for _, edge := range original.Workflow.Dependencies {
		if included[edge.ActivityID] && included[edge.DependsOnActivityID] {
			request.Workflow.Dependencies = append(request.Workflow.Dependencies, edge)
		}
	}
	for _, edge := range original.Workflow.DataDependencies {
		if included[edge.ConsumerActivityID] && included[edge.ProducerActivityID] {
			request.Workflow.DataDependencies = append(request.Workflow.DataDependencies, edge)
		}
	}
	for _, assignment := range original.Plan.Assignments {
		if included[assignment.ActivityID] {
			request.Plan.Assignments = append(request.Plan.Assignments, assignment)
		}
	}
	request.Plan.AssignmentCount = len(request.Plan.Assignments)

	if len(request.Workflow.Activities) != len(included) {
		return ports.ExecutionRequest{}, fmt.Errorf("the test includes an activity outside the original workflow")
	}
	if err := ValidateRequest(request); err != nil {
		return ports.ExecutionRequest{}, err
	}
	return request, nil
}

func (s *Supervisor) recordedWorkspaceSource(ctx context.Context, runID, activityID string) (domain.TransferLocation, error) {
	store := s.workspaceStore()
	if store == nil {
		return domain.TransferLocation{}, fmt.Errorf("recorded workspace storage is unavailable")
	}
	workspace, err := store.FindWorkspace(ctx, "workspace-"+runID+"-"+activityID)
	if err != nil {
		return domain.TransferLocation{}, err
	}
	if workspace == nil || workspace.RunID != runID || workspace.ActivityID != activityID ||
		workspace.SealedAt == nil || workspace.LastError != "" ||
		(workspace.State != domain.WorkspaceSealed && workspace.State != domain.WorkspaceReleasable) {
		return domain.TransferLocation{}, fmt.Errorf("recorded workspace for %q is unavailable", activityID)
	}
	stored, err := url.Parse(workspace.URI)
	if err != nil || stored.Scheme == "" {
		return domain.TransferLocation{}, fmt.Errorf("recorded workspace for %q has no usable location", activityID)
	}
	location := stored
	if stored.Scheme != "kubernetes" {
		suffix := "/" + path.Join(runID, activityID)
		if !path.IsAbs(workspace.ExecutionPath) || !strings.HasSuffix(path.Clean(workspace.ExecutionPath), suffix) {
			return domain.TransferLocation{}, fmt.Errorf("recorded workspace for %q has an invalid path", activityID)
		}
		location = &url.URL{Scheme: "file", Path: workspace.ExecutionPath}
	}
	if workspace.ConnectionID != "" {
		query := location.Query()
		query.Set("connectionId", workspace.ConnectionID)
		location.RawQuery = query.Encode()
	}
	return domain.TransferLocation{
		URI: location.String(), ConnectionID: workspace.ConnectionID, ResourceID: workspace.ResourceID,
		EnvironmentID: workspace.EnvironmentID, RuntimeID: workspace.RuntimeID,
		CloudInstanceID: stored.Query().Get("cloudInstanceId"),
	}, nil
}
