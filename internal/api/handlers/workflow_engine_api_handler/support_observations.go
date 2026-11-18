package workflow_engine_api_handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/supportbundle"
)

func (h *Handler) WithSupportObservations(collector *supportbundle.Collector) {
	h.supportObservations = collector
}

func (h *Handler) SupportObservations(w http.ResponseWriter, r *http.Request) {
	if h.supportObservations == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("support observations unavailable"))
		return
	}
	query := r.URL.Query()
	hours, err := strconv.Atoi(query.Get("hours"))
	if err != nil || (hours != 1 && hours != 24 && hours != 168) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("select a supported diagnostic time range"))
		return
	}
	options := map[string]bool{}
	for _, category := range strings.Split(query.Get("categories"), ",") {
		switch category {
		case "system", "configuration", "operations":
			options[category] = true
		default:
			writeError(w, http.StatusBadRequest, fmt.Errorf("select supported diagnostic categories"))
			return
		}
	}
	to := time.Now().UTC()
	if requested := query.Get("until"); requested != "" {
		at, err := time.Parse(time.RFC3339Nano, requested)
		if err != nil || at.After(to.Add(30*time.Second)) || at.Before(to.Add(-time.Minute)) {
			writeError(w, http.StatusBadRequest, fmt.Errorf("the diagnostic collection time is invalid"))
			return
		}
		to = at.UTC()
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, h.supportObservations.Collect(r.Context(), options, to.Add(-time.Duration(hours)*time.Hour), to))
}
