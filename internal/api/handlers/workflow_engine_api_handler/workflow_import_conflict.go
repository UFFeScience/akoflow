package workflow_engine_api_handler

import "net/http"

func (h *Handler) checkWorkflowImportConflict(w http.ResponseWriter, r *http.Request, id string) bool {
	existing, err := h.workflows.Find(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return false
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{
			"message":    "A workflow with this name already exists. Open it or choose a different name to import a separate copy.",
			"existingId": existing.ID,
		})
		return false
	}
	return true
}
