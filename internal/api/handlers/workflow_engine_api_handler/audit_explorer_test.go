package workflow_engine_api_handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
)

type auditExplorerStub struct {
	auditStub
	field, search string
	deadline      bool
}

func (s *auditExplorerStub) ListAuditFacet(ctx context.Context, filter domainaudit.Filter, field, search string, limit int) (domainaudit.FacetPage, error) {
	s.filter, s.field, s.search = filter, field, search
	_, s.deadline = ctx.Deadline()
	return domainaudit.FacetPage{Values: []domainaudit.FacetValue{{Value: "activity", Count: 230}}}, s.err
}
func (s *auditExplorerStub) SummarizeAudit(ctx context.Context, filter domainaudit.Filter) (domainaudit.Summary, error) {
	s.filter = filter
	_, s.deadline = ctx.Deadline()
	return domainaudit.Summary{Total: 230, Buckets: []domainaudit.TimeBucket{}}, s.err
}

func TestAuditExplorationHandlersValidateAndForwardFilters(t *testing.T) {
	store := &auditExplorerStub{}
	handler := &Handler{audit: store}
	for _, item := range []struct {
		target string
		handle http.HandlerFunc
		status int
	}{
		{"/?field=category&search=act&outcome=failed", handler.AuditFacets, 200},
		{"/?field=secret", handler.AuditFacets, 400},
		{"/?field=status&status=wrong", handler.AuditFacets, 400},
		{"/?from=2026-11-05T10:00:00Z&to=2026-11-05T12:00:00Z", handler.AuditSummary, 200},
		{"/?from=2026-11-05T12:00:00Z&to=2026-11-05T10:00:00Z", handler.AuditSummary, 400},
		{"/?from=invalid", handler.AuditSummary, 400},
	} {
		response := httptest.NewRecorder()
		item.handle(response, httptest.NewRequest("GET", item.target, nil))
		if response.Code != item.status {
			t.Fatalf("%s: %d %s", item.target, response.Code, response.Body.String())
		}
	}
	if store.field != "category" || store.search != "act" || !store.deadline {
		t.Fatalf("%+v", store)
	}
	store.err = errors.New("store unavailable")
	response := httptest.NewRecorder()
	handler.AuditSummary(response, httptest.NewRequest("GET", "/", nil))
	if response.Code != 500 {
		t.Fatalf("%d", response.Code)
	}
}

func TestAuditExplorationUnavailableDoesNotInventPageCounts(t *testing.T) {
	handler := &Handler{audit: &auditStub{}}
	for _, handle := range []http.HandlerFunc{handler.AuditFacets, handler.AuditSummary} {
		response := httptest.NewRecorder()
		handle(response, httptest.NewRequest("GET", "/?field=category", nil))
		if response.Code != 503 {
			t.Fatalf("%d", response.Code)
		}
	}
}
