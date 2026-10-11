package workflow_engine_api_handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/UFFeScience/akoflow/internal/domain/storagebudget"
)

func (h *Handler) WorkspaceStorageBudget(w http.ResponseWriter, r *http.Request) {
	observer, ok := h.storage.(interface {
		WorkspaceBudget(context.Context, string, string) (storagebudget.Snapshot, error)
	})
	if !ok {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("workspace quota observations are unavailable"))
		return
	}
	snapshot, err := observer.WorkspaceBudget(r.Context(), r.PathValue("storageId"), r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}
