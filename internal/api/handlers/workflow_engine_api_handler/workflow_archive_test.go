package workflow_engine_api_handler

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type archiveWorkflowStub struct {
	workflowRepositoryStub
	calls int
}

func (s *archiveWorkflowStub) SetArchived(_ context.Context, id string, archived bool) error {
	s.calls++
	if id != "workflow" {
		return sql.ErrNoRows
	}
	s.definition.Archived = archived
	return nil
}

func (s *archiveWorkflowStub) FindVersion(context.Context, string) (*domain.WorkflowVersion, error) {
	return &domain.WorkflowVersion{ID: "version", Archived: s.definition.Archived}, nil
}

func (s *archiveWorkflowStub) ListHeaders(context.Context) ([]domain.WorkflowDefinition, error) {
	return []domain.WorkflowDefinition{*s.definition}, nil
}

func TestArchiveWorkflowAPIValidationPersistenceAndReadOnly(t *testing.T) {
	for _, item := range []struct {
		body, id string
		readOnly bool
		status   int
		calls    int
	}{
		{`{"archived":true}`, "workflow", false, 200, 1},
		{`{"archived":false}`, "workflow", false, 200, 1},
		{`{}`, "workflow", false, 422, 0},
		{`{"archived":"yes"}`, "workflow", false, 400, 0},
		{`{"archived":true}`, "missing", false, 404, 1},
		{`{"archived":true}`, "workflow", true, 403, 0},
	} {
		t.Run(item.body+item.id+http.StatusText(item.status), func(t *testing.T) {
			store := &archiveWorkflowStub{workflowRepositoryStub: workflowRepositoryStub{definition: &domain.WorkflowDefinition{ID: "workflow"}}}
			h := &Handler{workflows: store, readOnly: item.readOnly}
			request := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(item.body))
			request.Header.Set("Content-Type", "application/json")
			request.SetPathValue("workflowId", item.id)
			response := httptest.NewRecorder()
			h.SetWorkflowArchived(response, request)
			if response.Code != item.status || store.calls != item.calls {
				t.Fatalf("response=%d %s calls=%d", response.Code, response.Body, store.calls)
			}
		})
	}
}

func TestArchivedWorkflowIsRemovedFromOptionsAndBlocksNewOperations(t *testing.T) {
	store := &archiveWorkflowStub{workflowRepositoryStub: workflowRepositoryStub{definition: &domain.WorkflowDefinition{ID: "workflow", Archived: true}}}
	h := &Handler{workflows: store}
	response := httptest.NewRecorder()
	h.listWorkflowOptions(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != 200 || strings.TrimSpace(response.Body.String()) != "[]" {
		t.Fatalf("options: %d %s", response.Code, response.Body)
	}
	for _, handler := range []http.HandlerFunc{h.CreatePlan, h.ImportPlan, h.CreateExecution} {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"plan":{"workflowVersionId":"version"}}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler(response, request)
		if response.Code != http.StatusConflict {
			t.Fatalf("archived operation response=%d %s", response.Code, response.Body)
		}
	}
}
