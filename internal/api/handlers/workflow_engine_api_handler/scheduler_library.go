package workflow_engine_api_handler

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/UFFeScience/akoflow/internal/planning/library"
)

func (h *Handler) WithAlgorithmLibrary(service *library.Service) { h.algorithmLibrary = service }
func (h *Handler) ListSchedulerLibrary(w http.ResponseWriter, r *http.Request) {
	if h.algorithmLibrary == nil {
		writeError(w, 503, fmt.Errorf("algorithm library is unavailable"))
		return
	}
	items, err := h.algorithmLibrary.List(r.Context())
	if err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, items)
}
func (h *Handler) GetSchedulerLibrary(w http.ResponseWriter, r *http.Request) {
	if h.algorithmLibrary == nil {
		writeError(w, 503, fmt.Errorf("algorithm library is unavailable"))
		return
	}
	item, err := h.algorithmLibrary.Get(r.Context(), r.PathValue("algorithmId"))
	if err != nil {
		status := 500
		if err == sql.ErrNoRows {
			status = 404
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, 200, item)
}
func (h *Handler) SchedulerTemplate(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"source": library.Template, "contract": library.Contract})
}
func (h *Handler) SaveSchedulerPlugin(w http.ResponseWriter, r *http.Request) {
	if h.readOnly {
		writeError(w, 403, fmt.Errorf("instance is read-only"))
		return
	}
	if h.algorithmLibrary == nil {
		writeError(w, 503, fmt.Errorf("algorithm library is unavailable"))
		return
	}
	var p library.Package
	if !decode(w, r, &p) {
		return
	}
	if id := r.PathValue("algorithmId"); id != "" && id != p.ID {
		writeError(w, 422, fmt.Errorf("algorithm id must match path"))
		return
	}
	if r.Method == http.MethodPost {
		_, err := h.algorithmLibrary.Get(r.Context(), p.ID)
		if err == nil {
			writeError(w, 409, fmt.Errorf("algorithm id already exists; edit it instead"))
			return
		}
		if err != sql.ErrNoRows {
			writeError(w, 500, err)
			return
		}
	}
	saved, err := h.algorithmLibrary.Save(r.Context(), p)
	if err != nil {
		writeError(w, 422, err)
		return
	}
	status := 200
	if r.Method == http.MethodPost {
		status = 201
	}
	writeJSON(w, status, saved)
}
