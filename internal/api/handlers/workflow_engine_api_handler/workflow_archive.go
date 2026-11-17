package workflow_engine_api_handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
)

func (h *Handler) SetWorkflowArchived(w http.ResponseWriter, r *http.Request) {
	if h.readOnly {
		writeError(w, http.StatusForbidden, fmt.Errorf("imported snapshots cannot change workflow status"))
		return
	}
	var input struct {
		Archived *bool `json:"archived"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Archived == nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("archived must be true or false"))
		return
	}
	store, ok := h.workflows.(interface {
		SetArchived(context.Context, string, bool) error
	})
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("workflow archiving is unavailable"))
		return
	}
	id := r.PathValue("workflowId")
	if err := store.SetArchived(r.Context(), id, *input.Archived); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, fmt.Errorf("workflow not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	item, err := h.workflows.Find(r.Context(), id)
	writeItem(w, item, err)
}

func (h *Handler) rejectArchivedWorkflow(w http.ResponseWriter, r *http.Request, versionID string) bool {
	if h.workflows == nil || versionID == "" {
		return false
	}
	workflow, err := h.workflows.FindVersion(r.Context(), versionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return true
	}
	if workflow != nil && workflow.Archived {
		writeError(w, http.StatusConflict, fmt.Errorf("this workflow is archived; restore it before planning or starting a new execution"))
		return true
	}
	return false
}
