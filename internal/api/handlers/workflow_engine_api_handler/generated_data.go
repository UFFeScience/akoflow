package workflow_engine_api_handler

import (
	"context"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/domain"
	"net/http"
)

func (h *Handler) ListGeneratedData(w http.ResponseWriter, r *http.Request) {
	query, ok := h.data.(interface {
		ListGeneratedDataPage(context.Context, int, int) (domain.GeneratedDataPage, error)
	})
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("generated data pagination is unavailable"))
		return
	}
	result, err := query.ListGeneratedDataPage(r.Context(), positiveInteger(r.URL.Query().Get("page"), 1), positiveInteger(r.URL.Query().Get("pageSize"), 20))
	writeList(w, result, err)
}
