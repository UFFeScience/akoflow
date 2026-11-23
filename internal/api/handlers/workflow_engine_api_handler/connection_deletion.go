package workflow_engine_api_handler

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/UFFeScience/akoflow/internal/application/ports"
)

func (h *Handler) EnvironmentConnectionDeletion(w http.ResponseWriter, r *http.Request) {
	store, ok := h.environments.(ports.ConnectionDeletionStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("connection deletion is unavailable"))
		return
	}

	id := r.PathValue("connectionId")
	action := store.PreviewConnectionDeletion
	if r.Method == http.MethodDelete {
		action = store.DeleteConnection
	}
	result, err := action(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, fmt.Errorf("connection %q was not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, result)
		return
	}
	if len(result.Dependencies) > 0 {
		writeJSON(w, http.StatusConflict, struct {
			ports.ConnectionDeletion
			Error string `json:"error"`
		}{result, "This connection is still in use."})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
