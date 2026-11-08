package workflow_engine_api_handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	domainsearch "github.com/UFFeScience/akoflow/internal/domain/search"
)

type searchStub struct {
	query    domainsearch.Query
	deadline bool
	err      error
}

func (s *searchStub) Search(ctx context.Context, q domainsearch.Query) (domainsearch.Page, error) {
	s.query = q
	_, s.deadline = ctx.Deadline()
	return domainsearch.Page{Query: q.Text, Results: []domainsearch.Result{{Type: "workflow", ID: "wf", Title: "Science"}}, Total: 1}, s.err
}

func TestSearchDelegatesBoundedQueryWithDeadline(t *testing.T) {
	store := &searchStub{}
	h := &Handler{search: store}
	response := callHandler(t, http.MethodGet, "/?q=science&types=workflow,workflow&limit=5&cursor=next", "", nil, h.Search)
	if response.Code != 200 || store.query.Limit != 5 || len(store.query.Types) != 1 || store.query.Cursor != "next" || !store.deadline {
		t.Fatalf("response %d, query %+v, deadline %v", response.Code, store.query, store.deadline)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("search response may be cached across instances")
	}
	for _, query := range []string{"?limit=0", "?limit=51", "?limit=bad", "?types=credentials", "?q=" + strings.Repeat("a", 129)} {
		response = callHandler(t, http.MethodGet, "/"+query, "", nil, h.Search)
		if response.Code != 400 {
			t.Fatalf("query %s: %d", query, response.Code)
		}
	}
}

func TestSearchHandlesEmptyUnavailableAndCanceledQueries(t *testing.T) {
	h := &Handler{}
	response := callHandler(t, http.MethodGet, "/?q=", "", nil, h.Search)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"results":[]`) {
		t.Fatal(response.Body.String())
	}
	response = callHandler(t, http.MethodGet, "/?q=science", "", nil, h.Search)
	if response.Code != 503 {
		t.Fatal(response.Code)
	}
	for _, failure := range []struct {
		err    error
		status int
	}{
		{domainsearch.ErrInvalidCursor, 400}, {context.DeadlineExceeded, 504}, {errors.New("private database details"), 503},
	} {
		h.search = &searchStub{err: failure.err}
		response = callHandler(t, http.MethodGet, "/?q=science", "", nil, h.Search)
		if response.Code != failure.status || strings.Contains(response.Body.String(), "private database") {
			t.Fatal(response.Code, response.Body.String())
		}
	}
}
