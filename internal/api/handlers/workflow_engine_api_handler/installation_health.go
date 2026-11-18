package workflow_engine_api_handler

import (
	"fmt"
	"net/http"

	"github.com/UFFeScience/akoflow/internal/application/installationhealth"
)

func (h *Handler) WithInstallationHealth(checks *installationhealth.Service) {
	h.installationHealth = checks
}

func (h *Handler) InstallationHealth(w http.ResponseWriter, r *http.Request) {
	if h.installationHealth == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("installation health is unavailable"))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, h.installationHealth.Snapshot(r.Context(), r.URL.Query().Get("refresh") == "1"))
}
