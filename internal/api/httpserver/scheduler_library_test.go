package httpserver

import (
	"context"

	"database/sql"
	"encoding/json"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/api/handlers/workflow_engine_api_handler"
	"github.com/UFFeScience/akoflow/internal/planning/algorithms"
	"github.com/UFFeScience/akoflow/internal/planning/library"
	_ "github.com/mattn/go-sqlite3"
)

func TestSchedulerLibraryRoutesAndNativeProtection(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Bootstrap(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	registry, _ := algorithms.NewRegistry(algorithms.HEFT{}, algorithms.NewPRISMTime(), algorithms.NewPRISMCost())
	service, err := library.New(context.Background(), db, registry, t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	handler := &workflow_engine_api_handler.Handler{}
	handler.WithAlgorithmLibrary(service)
	mux := NewMux(handler)
	for _, path := range []string{"/akoflow-api/scheduling-algorithms/", "/akoflow-api/scheduling-algorithms/template/", "/akoflow-api/scheduling-algorithms/prism-time/"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		var body any
		if err = json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/akoflow-api/scheduling-algorithms/prism-time/", strings.NewReader(`{"id":"prism-time","name":"Override","source":"bad source","objective":"time"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != 422 {
		t.Fatalf("native override response: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/akoflow-api/scheduling-algorithms/missing/", nil))
	if rec.Code != 404 {
		t.Fatalf("missing response: %d", rec.Code)
	}
}
