package workflow_engine_api_handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
)

type provenanceStub struct {
	query ports.ProvenanceQuery
}

func (s *provenanceStub) Entities() []ports.ProvenanceEntity {
	return []ports.ProvenanceEntity{{Name: "runs", Label: "Runs"}}
}

func (s *provenanceStub) Query(_ context.Context, query ports.ProvenanceQuery) (ports.ProvenancePage, error) {
	s.query = query
	return ports.ProvenancePage{
		Entity: ports.ProvenanceEntity{Name: "runs", Label: "Runs"},
		Items:  []map[string]any{{"id": "run-1"}},
		Page:   query.Page,
	}, nil
}

func TestListProvenanceEntities(t *testing.T) {
	recorder := httptest.NewRecorder()
	(&Handler{provenance: &provenanceStub{}}).ListProvenanceEntities(
		recorder,
		httptest.NewRequest(http.MethodGet, "/provenance/entities/", nil),
	)
	if recorder.Code != http.StatusOK || recorder.Body.String() == "" {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestQueryProvenanceEntityForwardsSafeParameters(t *testing.T) {
	stub := &provenanceStub{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/provenance/entities/runs/?q=complete&filterField=status&filterValue=completed&page=2&pageSize=25",
		nil,
	)
	request.SetPathValue("entity", "runs")
	(&Handler{provenance: stub}).QueryProvenanceEntity(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
	if stub.query.Entity != "runs" || stub.query.FilterField != "status" || stub.query.Page != 2 {
		t.Fatalf("unexpected query: %#v", stub.query)
	}
}
