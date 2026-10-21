package workflow_engine_api_handler

import (
	"context"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainevents "github.com/UFFeScience/akoflow/internal/domain/events"
	"net/http"
)

type executionHandleReader interface {
	ListHandlesView(context.Context, string, string, bool) ([]domain.ActivityHandle, error)
	ListTasksForActivity(context.Context, string, string) ([]domain.TaskExecution, error)
}

func (h *Handler) executionHandles(ctx context.Context, runID, activityID string, output bool) ([]domain.ActivityHandle, error) {
	if reader, ok := h.executions.(executionHandleReader); ok {
		return reader.ListHandlesView(ctx, runID, activityID, output)
	}
	handles, err := h.executions.ListHandles(ctx, runID)
	result := make([]domain.ActivityHandle, 0)
	for _, handle := range handles {
		if activityID != "" && handle.ActivityID != activityID {
			continue
		}
		if !output {
			handle.Log = ""
			handle.Artifacts = nil
		}
		result = append(result, handle)
	}
	return result, err
}

func (h *Handler) executionEvents(ctx context.Context, runID string, include bool) ([]domainevents.Event, error) {
	if !include {
		return []domainevents.Event{}, nil
	}
	return h.executions.ListEvents(ctx, runID)
}

func (h *Handler) infrastructureEvents(ctx context.Context, operationID string, skip bool) ([]domain.CloudOperationEvent, error) {
	if skip {
		return []domain.CloudOperationEvent{}, nil
	}
	return h.cloudOperations.ListCloudOperationEvents(ctx, operationID)
}

// Narrow views avoid loading output and data-plane catalogs just to inspect status.
func (h *Handler) executionReadView(w http.ResponseWriter, r *http.Request, run *domain.ExecutionRun) bool {
	switch r.URL.Query().Get("view") {
	case "status":
		writeJSON(w, http.StatusOK, map[string]any{"run": run})
	case "activity":
		h.getExecutionActivity(w, r, run)
	case "logs":
		h.getExecutionLogs(w, r, run)
	case "data":
		h.getExecutionData(w, r, run)
	case "events":
		h.getExecutionEvents(w, r, run)
	default:
		return false
	}
	return true
}

func (h *Handler) getExecutionActivity(w http.ResponseWriter, r *http.Request, run *domain.ExecutionRun) {
	activityID := r.URL.Query().Get("activityId")
	if activityID == "" {
		writeError(w, http.StatusBadRequest, nil)
		return
	}
	reader, ok := h.executions.(executionHandleReader)
	if !ok {
		writeError(w, http.StatusNotImplemented, nil)
		return
	}
	tasks, err := reader.ListTasksForActivity(r.Context(), run.ID, activityID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	handles, err := h.executionHandles(r.Context(), run.ID, activityID, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "activities": latestTaskAttempts(tasks), "handles": handles})
}

func (h *Handler) getExecutionLogs(w http.ResponseWriter, r *http.Request, run *domain.ExecutionRun) {
	handles, err := h.processHandles(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	// Manifests are independent of process output and can be much larger than logs.
	for i := range handles {
		handles[i].Artifacts = nil
	}
	events, err := h.executions.ListEvents(r.Context(), run.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_, infrastructure, err := h.executionInfrastructure(r.Context(), run.ID, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	response := map[string]any{"run": run, "handles": handles, "events": events, "infrastructureRuns": infrastructure}
	if reader, ok := h.data.(WorkflowOperationEventQuery); ok {
		operations, err := reader.ListWorkflowOperationEvents(r.Context(), run.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		response["operationEvents"] = operations
	}
	writeJSON(w, http.StatusOK, response)
}

type executionEventPager interface {
	ListEventsPage(context.Context, string, int, int) ([]domainevents.Event, bool, error)
}

func (h *Handler) getExecutionEvents(w http.ResponseWriter, r *http.Request, run *domain.ExecutionRun) {
	reader, ok := h.executions.(executionEventPager)
	if !ok {
		writeError(w, http.StatusNotImplemented, nil)
		return
	}
	page, size := positiveInteger(r.URL.Query().Get("page"), 1), positiveInteger(r.URL.Query().Get("pageSize"), 100)
	if size > 200 {
		size = 200
	}
	events, more, err := reader.ListEventsPage(r.Context(), run.ID, page, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events, "page": page, "hasNext": more})
}

func (h *Handler) addMonitorWorkspaces(ctx context.Context, runID string, response map[string]any, monitor bool) error {
	if monitor {
		return nil
	}
	return h.addExecutionWorkspaces(ctx, runID, response)
}

func (h *Handler) addExecutionFileCatalog(ctx context.Context, runID string, response map[string]any) error {
	if h.data == nil {
		return nil
	}
	instances, err := h.data.ListInstances(ctx, runID)
	if err != nil {
		return err
	}
	locations, err := h.data.ListLocations(ctx, runID)
	if err != nil {
		return err
	}
	materializations, err := h.data.ListArtifactMaterializations(ctx, runID)
	if err != nil {
		return err
	}
	response["dataObjects"], response["dataLocations"] = instances, locations
	response["artifactMaterializations"] = materializations
	return nil
}

func (h *Handler) getExecutionData(w http.ResponseWriter, r *http.Request, run *domain.ExecutionRun) {
	response := map[string]any{}
	if err := h.addExecutionWorkspaces(r.Context(), run.ID, response); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := h.addExecutionFileCatalog(r.Context(), run.ID, response); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

type cloudOperationPager interface {
	filteredCloudOperations
	CountCloudOperations(context.Context, domain.CloudOperationFilter) (int, error)
}

func (h *Handler) listCloudOperationsPage(w http.ResponseWriter, r *http.Request) {
	reader, ok := h.cloudOperations.(cloudOperationPager)
	if !ok {
		writeError(w, http.StatusNotImplemented, nil)
		return
	}
	page := positiveInteger(r.URL.Query().Get("page"), 1)
	size := min(100, positiveInteger(r.URL.Query().Get("pageSize"), 20))
	filter := domain.CloudOperationFilter{Status: r.URL.Query().Get("status"), Limit: size, Offset: (page - 1) * size}
	total, err := reader.CountCloudOperations(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	items, err := reader.ListCloudOperationsFiltered(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page": page, "total": total, "hasNext": filter.Offset+len(items) < total})
}

func (h *Handler) processHandles(ctx context.Context, runID string) ([]domain.ActivityHandle, error) {
	if reader, ok := h.executions.(interface {
		ListProcessHandles(context.Context, string) ([]domain.ActivityHandle, error)
	}); ok {
		return reader.ListProcessHandles(ctx, runID)
	}
	return h.executionHandles(ctx, runID, "", true)
}
