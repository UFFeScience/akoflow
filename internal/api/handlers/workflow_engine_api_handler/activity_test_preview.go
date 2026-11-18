package workflow_engine_api_handler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/controlplane/eventloop"
	executioncontrol "github.com/UFFeScience/akoflow/internal/controlplane/execution"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type activityTestInput struct {
	Path      string `json:"path"`
	Digest    string `json:"digest,omitempty"`
	SizeBytes int64  `json:"sizeBytes"`
	Producer  string `json:"producerActivityId,omitempty"`
}

type activityTestPreview struct {
	SourceRunID string `json:"sourceRunId"`
	ActivityID  string `json:"activityId"`
	Attempt     int    `json:"attempt"`
	executioncontrol.ActivityTestGraph
	Inputs   []activityTestInput `json:"inputs"`
	Missing  []string            `json:"missing"`
	Revision string              `json:"revision"`
	request  ports.ExecutionRequest
	recorded map[string][]ports.RecordedWorkspaceInput
}

func (h *Handler) GetActivityTestPreview(w http.ResponseWriter, r *http.Request) {
	runID, activityID := r.PathValue("runId"), r.PathValue("activityId")
	attempt := 0
	if value := r.URL.Query().Get("attempt"); value != "" {
		var err error
		attempt, err = strconv.Atoi(value)
		if err != nil || attempt < 1 {
			writeError(w, http.StatusBadRequest, fmt.Errorf("attempt must be a positive integer"))
			return
		}
	}
	preview, err := h.buildActivityTestPreview(r.Context(), runID, activityID, attempt)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *Handler) buildActivityTestPreview(ctx context.Context, runID, activityID string, attempt int) (activityTestPreview, error) {
	var empty activityTestPreview
	queue, ok := h.queue.(ExecutionRecoveryQueueStore)
	if !ok {
		return empty, fmt.Errorf("execution history is unavailable")
	}
	run, err := h.executions.FindRun(ctx, runID)
	if err != nil {
		return empty, err
	}
	if run == nil || run.Mode != domain.ExecutionModeReal {
		return empty, fmt.Errorf("a recorded real execution is required")
	}
	if run.Status != domain.ExecutionRunCompleted && run.Status != domain.ExecutionRunFailed &&
		run.Status != domain.ExecutionRunCancelled {
		return empty, fmt.Errorf("wait for the original execution to finish before testing an activity")
	}
	job, err := queue.FindLatestByAggregate(ctx, "execution_run", runID, eventloop.EventExecutionRunRequested)
	if err != nil {
		return empty, err
	}
	if job == nil {
		return empty, fmt.Errorf("the original execution request is unavailable")
	}
	var request ports.ExecutionRequest
	if err := json.Unmarshal(job.Payload, &request); err != nil {
		return empty, fmt.Errorf("decode original execution request: %w", err)
	}
	selectedAttempt, manifest, err := h.recordedActivityAttempt(ctx, runID, activityID, attempt)
	if err != nil {
		return empty, err
	}
	var workspaces []domain.ActivityWorkspace
	if store, ok := h.executions.(WorkspaceQuery); ok {
		workspaces, err = store.ListWorkspaces(ctx, runID)
		if err != nil {
			return empty, err
		}
	}
	preview, err := describeActivityTest(request.Workflow, runID, activityID, selectedAttempt, manifest, workspaces)
	if err != nil {
		return empty, err
	}
	verification, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	changed := false
	for i := range workspaces {
		workspace := &workspaces[i]
		if !slices.Contains(preview.ReuseActivityIDs, workspace.ActivityID) {
			continue
		}
		if h.workspaceVerifier == nil || h.workspaceVerifier.VerifyRecorded(verification, *workspace) != nil {
			workspace.LastError = "recorded files unavailable"
			changed = true
		}
	}
	if changed {
		preview, err = describeActivityTest(request.Workflow, runID, activityID, selectedAttempt, manifest, workspaces)
		if err != nil {
			return empty, err
		}
	}
	for _, probe := range h.preflightInputs(request) {
		if !slices.Contains(preview.RunActivityIDs, probe.EntityID) {
			continue
		}
		if status, detail, _ := probe.Run(ctx); status == "blocked" {
			preview.Missing = append(preview.Missing, fmt.Sprintf("%s: %s", probe.EntityID, detail))
		}
	}
	preview.request = request
	payload, err := json.Marshal(preview)
	if err != nil {
		return empty, err
	}
	preview.Revision = fmt.Sprintf("%x", sha256.Sum256(payload))
	return preview, nil
}

func (h *Handler) recordedActivityAttempt(ctx context.Context, runID, activityID string, attempt int) (int, *domain.ArtifactManifest, error) {
	tasks, err := h.executions.ListTasks(ctx, runID)
	if err != nil {
		return 0, nil, err
	}
	var selected *domain.TaskExecution
	for _, task := range tasks {
		if task.ActivityID == activityID && (attempt == 0 || task.Attempt == attempt) &&
			(selected == nil || task.Attempt > selected.Attempt) {
			copy := task
			selected = &copy
		}
	}
	if selected == nil || selected.StartedAt == 0 {
		return 0, nil, fmt.Errorf("the selected activity attempt did not start")
	}
	handles, err := h.executions.ListHandles(ctx, runID)
	if err != nil {
		return 0, nil, err
	}
	var manifest *domain.ArtifactManifest
	for _, handle := range handles {
		if item := handle.Artifacts; item != nil && item.RunID == runID &&
			item.ActivityID == activityID && item.Attempt == selected.Attempt {
			manifest = item
			break
		}
	}
	return selected.Attempt, manifest, nil
}

func describeActivityTest(workflow domain.WorkflowVersion, runID, activityID string, attempt int, manifest *domain.ArtifactManifest, workspaces []domain.ActivityWorkspace) (activityTestPreview, error) {
	preview := activityTestPreview{SourceRunID: runID, ActivityID: activityID, Attempt: attempt,
		Inputs: []activityTestInput{}, Missing: []string{}}
	producers := recoveryProducerIDs(workflow, activityID)
	reusable := make(map[string]bool)
	recorded := make(map[string][]domain.WorkspaceEntry)
	candidates := make(map[domain.WorkspaceEntry]string)
	if manifest == nil {
		preview.Missing = append(preview.Missing, "The attempt has no recorded input snapshot.")
	}
	for _, workspace := range workspaces {
		if workspace.RunID != runID || workspace.SealedAt == nil || workspace.LastError != "" ||
			(workspace.State != domain.WorkspaceSealed && workspace.State != domain.WorkspaceReleasable) {
			continue
		}
		for _, producer := range producers {
			if producer != workspace.ActivityID || !matchesAttemptInputs(workspace, manifest) {
				continue
			}
			reusable[producer] = true
			recorded[producer] = workspace.Manifest.Final
			for _, entry := range workspace.Manifest.Final {
				if existing, ok := candidates[entry]; !ok || producer < existing {
					candidates[entry] = producer
				}
			}
		}
	}
	if manifest != nil {
		for _, entry := range manifest.InitialSnapshot {
			producer := candidates[domain.WorkspaceEntry{Path: entry.Path, Digest: entry.Checksum, SizeBytes: entry.SizeBytes}]
			preview.Inputs = append(preview.Inputs, activityTestInput{Path: entry.Path, Digest: entry.Checksum,
				SizeBytes: entry.SizeBytes, Producer: producer})
			if producer == "" {
				preview.Missing = append(preview.Missing, entry.Path)
			}
		}
	}
	graph, err := executioncontrol.PlanActivityTestGraph(workflow, activityID, reusable)
	preview.ActivityTestGraph = graph
	preview.recorded = make(map[string][]ports.RecordedWorkspaceInput)
	for _, consumer := range graph.RunActivityIDs {
		for _, producer := range recoveryProducerIDs(workflow, consumer) {
			if entries := recorded[producer]; len(entries) > 0 && slices.Contains(graph.ReuseActivityIDs, producer) {
				preview.recorded[consumer] = append(preview.recorded[consumer], ports.RecordedWorkspaceInput{
					ProducerActivityID: producer, Entries: entries,
				})
			}
		}
	}
	return preview, err
}

func matchesAttemptInputs(workspace domain.ActivityWorkspace, manifest *domain.ArtifactManifest) bool {
	final := workspace.Manifest.Final
	if manifest == nil || len(final) == 0 || int64(len(final)) != workspace.FileCount {
		return false
	}
	inputs := make(map[domain.WorkspaceEntry]bool, len(manifest.InitialSnapshot))
	for _, entry := range manifest.InitialSnapshot {
		inputs[domain.WorkspaceEntry{Path: entry.Path, Digest: entry.Checksum, SizeBytes: entry.SizeBytes}] = true
	}
	for _, entry := range final {
		if entry.Digest == "" || !inputs[entry] {
			return false
		}
	}
	return true
}
