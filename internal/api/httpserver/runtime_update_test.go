package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRuntimeUpdateGateReservesOnlyIdleDaemon(t *testing.T) {
	active := true
	gate := NewRuntimeUpdateGate(func(context.Context) (bool, error) { return active, nil }, false)
	handler := gate.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}), "test-token")
	request := func(method, path, token string) int {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, req)
		return result.Code
	}
	check := func(method, path, token string, want int) {
		t.Helper()
		if got := request(method, path, token); got != want {
			t.Fatalf("%s %s: got %d, want %d", method, path, got, want)
		}
	}
	check("POST", runtimeUpdatePath, "invalid", http.StatusUnauthorized)
	check("POST", runtimeUpdatePath, "test-token", http.StatusConflict)
	active = false
	if !gate.Admit() {
		t.Fatal("expected to admit scheduled work")
	}
	check("POST", runtimeUpdatePath, "test-token", http.StatusConflict)
	gate.Done()
	check("POST", runtimeUpdatePath, "test-token", http.StatusNoContent)
	check("POST", "/akoflow-api/execution-runs/", "test-token", http.StatusServiceUnavailable)
	check("GET", "/akoflow-api/instance/", "test-token", http.StatusAccepted)
	check("DELETE", runtimeUpdatePath, "test-token", http.StatusNoContent)
	check("POST", "/akoflow-api/execution-runs/", "test-token", http.StatusAccepted)
}
