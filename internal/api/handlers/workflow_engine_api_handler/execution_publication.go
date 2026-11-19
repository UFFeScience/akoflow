package workflow_engine_api_handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/controlplane/eventloop"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func (h *Handler) getExecutedWorkflow(w http.ResponseWriter, r *http.Request, run *domain.ExecutionRun) {
	queue, ok := h.queue.(ExecutionRecoveryQueueStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("recorded execution requests are unavailable"))
		return
	}
	ctx := r.Context()
	job, err := queue.FindLatestByAggregate(ctx, "execution_run", run.ID, eventloop.EventExecutionRunRequested)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var request ports.ExecutionRequest
	if job == nil || json.Unmarshal(job.Payload, &request) != nil || request.Run.ID != run.ID ||
		request.ActivityTest != nil || request.Plan.ID != run.SchedulePlanID ||
		request.Workflow.ID == "" || request.Plan.WorkflowVersionID != request.Workflow.ID {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("the executed workflow definition is unavailable"))
		return
	}
	definition, err := h.planWorkflow(ctx, request.Workflow.ID)
	if err != nil || definition == nil {
		writeItem(w, definition, err)
		return
	}
	definition.Version = request.Workflow
	writeJSON(w, http.StatusOK, definition)
}
