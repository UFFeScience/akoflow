package workflow_engine_api_handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	applicationexecution "github.com/UFFeScience/akoflow/internal/application/execution"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/controlplane/eventloop"
	executioncontrol "github.com/UFFeScience/akoflow/internal/controlplane/execution"
	"github.com/google/uuid"
)

func (h *Handler) CreateActivityTest(w http.ResponseWriter, r *http.Request) {
	if h.readOnly {
		writeError(w, http.StatusForbidden, fmt.Errorf("read-only instances cannot run activity tests"))
		return
	}
	store, ok := h.executionCommands.(ExecutionSubmissionStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("execution submission is unavailable"))
		return
	}
	var input struct {
		Attempt      int    `json:"attempt"`
		Revision     string `json:"revision"`
		SubmissionID string `json:"submissionId"`
	}
	if !decode(w, r, &input) {
		return
	}
	submissionID, err := uuid.Parse(input.SubmissionID)
	if err != nil || submissionID == uuid.Nil || input.Attempt < 1 || input.Revision == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("attempt, review revision and submission identifier are required"))
		return
	}
	ctx := r.Context()
	preview, err := h.buildActivityTestPreview(ctx, r.PathValue("runId"), r.PathValue("activityId"), input.Attempt)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if preview.Revision != input.Revision || len(preview.Missing) != 0 {
		writeError(w, http.StatusConflict, fmt.Errorf("the original inputs are missing or have changed; review the activity test again"))
		return
	}
	if h.rejectArchivedWorkflow(w, r, preview.request.Workflow.ID) {
		return
	}

	runID := "activity-test-" + submissionID.String()
	request, err := executioncontrol.ProjectActivityTestRequest(preview.request, runID, preview.ActivityTestGraph)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	request.ActivityTest = &ports.ActivityTestSource{
		RunID: preview.SourceRunID, ActivityID: preview.ActivityID,
		Attempt: preview.Attempt, Inputs: preview.recorded,
	}
	resolveDirectOCIImages(&request.Workflow)
	job, err := applicationexecution.NewExecutionJob(request, "activity-test:"+submissionID.String(), time.Now().UTC())
	if err == nil {
		_, err = store.SubmitRun(ctx, request.Run, job)
	}
	if err != nil {
		status := http.StatusUnprocessableEntity
		if errors.Is(err, ports.ErrExecutionSubmissionConflict) {
			status = http.StatusConflict
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"runId": runID})
}

func (h *Handler) addActivityTestContext(ctx context.Context, runID string, response map[string]any) {
	if !strings.HasPrefix(runID, "activity-test-") {
		return
	}
	queue, ok := h.queue.(ExecutionRecoveryQueueStore)
	if !ok {
		return
	}
	job, err := queue.FindLatestByAggregate(ctx, "execution_run", runID, eventloop.EventExecutionRunRequested)
	if err != nil || job == nil {
		return
	}
	var request ports.ExecutionRequest
	if json.Unmarshal(job.Payload, &request) != nil || request.ActivityTest == nil {
		return
	}
	ids := make([]string, 0, len(request.Workflow.Activities))
	for _, activity := range request.Workflow.Activities {
		ids = append(ids, activity.ID)
	}
	response["activityTest"] = map[string]any{
		"sourceRunId": request.ActivityTest.RunID, "targetActivityId": request.ActivityTest.ActivityID,
		"attempt": request.ActivityTest.Attempt, "activityIds": ids,
	}
}
