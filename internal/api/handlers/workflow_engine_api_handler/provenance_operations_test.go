package workflow_engine_api_handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
)

type provenanceStub struct {
	query        ports.ProvenanceQuery
	sqlQuery     ports.ProvenanceSQLQuery
	lineageQuery ports.ProvenanceLineageQuery
}

func (s *provenanceStub) SQL(_ context.Context, query ports.ProvenanceSQLQuery) (ports.ProvenanceSQLResult, error) {
	s.sqlQuery = query
	return ports.ProvenanceSQLResult{Items: []map[string]any{{"id": "run-1"}}}, nil
}

func (s *provenanceStub) Explain(_ context.Context, query ports.ProvenanceSQLQuery) (ports.ProvenanceSQLResult, error) {
	s.sqlQuery = query
	return ports.ProvenanceSQLResult{Items: []map[string]any{{"detail": "SCAN runs"}}}, nil
}

func (s *provenanceStub) Lineage(_ context.Context, query ports.ProvenanceLineageQuery) (ports.ProvenanceLineage, error) {
	s.lineageQuery = query
	return ports.ProvenanceLineage{Root: "runs:" + query.ID}, nil
}

func (s *provenanceStub) Entities() []ports.ProvenanceEntity {
	return []ports.ProvenanceEntity{{Name: "runs", Label: "Runs"}}
}

func (s *provenanceStub) Schema(context.Context) ([]ports.ProvenanceSQLTable, error) {
	return []ports.ProvenanceSQLTable{{Name: "execution_runs"}}, nil
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
