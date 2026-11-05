package workflow_engine_api_handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
)

func parseAuditFilter(r *http.Request) (domainaudit.Filter, error) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	status, statusErr := strconv.Atoi(r.URL.Query().Get("status"))
	if r.URL.Query().Get("status") != "" && statusErr != nil {
		return domainaudit.Filter{}, fmt.Errorf("invalid audit status")
	}
	filter := domainaudit.Filter{Category: r.URL.Query().Get("category"), Status: status,
		RequestID:     r.URL.Query().Get("requestId"),
		CorrelationID: r.URL.Query().Get("correlationId"),
		ActorID:       r.URL.Query().Get("actorId"),
		Method:        r.URL.Query().Get("method"),
		Route:         r.URL.Query().Get("route"),
		Transport:     r.URL.Query().Get("transport"),
		RuntimeID:     r.URL.Query().Get("runtimeId"),
		ActivityID:    r.URL.Query().Get("activityId"),
		OperationID:   r.URL.Query().Get("operationId"),
		Before:        r.URL.Query().Get("before"),
		From:          r.URL.Query().Get("from"),
		To:            r.URL.Query().Get("to"),
		EventType:     r.URL.Query().Get("eventType"),
		EnvironmentID: r.URL.Query().Get("environmentId"),
		ResourceID:    r.URL.Query().Get("resourceId"), ConnectionID: r.URL.Query().Get("connectionId"),
		SessionID: r.URL.Query().Get("sessionId"), ExecutionID: r.URL.Query().Get("executionId"),
		Outcome: domainaudit.Outcome(r.URL.Query().Get("outcome")), Limit: limit}
	for _, value := range []string{filter.From, filter.To} {
		if value != "" {
			if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
				return domainaudit.Filter{}, fmt.Errorf("audit dates must use RFC3339")
			}
		}
	}
	if filter.Status != 0 && (filter.Status < 100 || filter.Status > 599) {
		return domainaudit.Filter{}, fmt.Errorf("invalid audit status")
	}

	return filter, nil
}

func (h *Handler) AuditFacets(w http.ResponseWriter, r *http.Request) {
	explorer, ok := h.audit.(ports.AuditExplorer)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("audit exploration is unavailable"))
		return
	}
	filter, err := parseAuditFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	field := r.URL.Query().Get("field")
	if !validAuditFacet(field) || len(r.URL.Query().Get("search")) > 256 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported audit facet"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := explorer.ListAuditFacet(ctx, filter, field, r.URL.Query().Get("search"), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) AuditSummary(w http.ResponseWriter, r *http.Request) {
	explorer, ok := h.audit.(ports.AuditExplorer)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("audit exploration is unavailable"))
		return
	}
	filter, err := parseAuditFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if filter.From != "" && filter.To != "" {
		from, _ := time.Parse(time.RFC3339Nano, filter.From)
		to, _ := time.Parse(time.RFC3339Nano, filter.To)
		if !from.Before(to) {
			writeError(w, http.StatusBadRequest, fmt.Errorf("audit range must end after it starts"))
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	summary, err := explorer.SummarizeAudit(ctx, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func validAuditFacet(field string) bool {
	switch field {
	case "category", "eventType", "outcome", "transport", "actorId", "method", "status", "route",
		"environmentId", "resourceId", "connectionId", "runtimeId", "sessionId", "executionId",
		"activityId", "operationId", "requestId", "correlationId":
		return true
	default:
		return false
	}
}
