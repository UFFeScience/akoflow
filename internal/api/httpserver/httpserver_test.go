package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UFFeScience/akoflow/internal/api/handlers/workflow_engine_api_handler"
)

func TestHealthCheck(t *testing.T) {
	recorder := httptest.NewRecorder()
	HealthCheck(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "ok" {
		t.Fatalf("unexpected response: %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestAllowCORS(t *testing.T) {
	nextCalls := 0
	handler := AllowCORSFor(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { nextCalls++; w.WriteHeader(http.StatusCreated) }), []string{"http://localhost:3000"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated || nextCalls != 1 || recorder.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatal("CORS GET failed")
	}
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodOptions, "/", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || nextCalls != 1 {
		t.Fatal("OPTIONS must short-circuit")
	}
}

func TestAllowCORSRejectsUnknownPreflightAndLeavesSameOriginUntouched(t *testing.T) {
	calls := 0
	handler := AllowCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++; w.WriteHeader(http.StatusAccepted) }))
	request := httptest.NewRequest(http.MethodOptions, "/", nil)
	request.Header.Set("Origin", "https://unknown.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || calls != 0 {
		t.Fatalf("preflight = %d calls=%d", response.Code, calls)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusAccepted || response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("same-origin = %d %#v", response.Code, response.Header())
	}
}

func TestPreflightAlwaysReportsServerAndDependencyState(t *testing.T) {
	response := httptest.NewRecorder()
	Preflight(response, httptest.NewRequest(http.MethodGet, "/akoflow-api/preflight/", nil))
	var payload map[string]map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["server"]["available"] != true || payload["docker"] == nil || payload["buildkit"] == nil {
		t.Fatalf("preflight = %#v", payload)
	}
}

func TestNewMuxRegistersHealthAndRejectsWrongMethods(t *testing.T) {
	mux := NewMux(&workflow_engine_api_handler.Handler{})
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("health = %d %q", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method = %d", response.Code)
	}
}

func TestLoopbackAddressRecognition(t *testing.T) {
	for _, address := range []string{"localhost:8080", "127.0.0.1:8080", "[::1]:8080"} {
		if !isLoopbackAddress(address) {
			t.Fatalf("%q should be loopback", address)
		}
	}
	for _, address := range []string{"0.0.0.0:8080", "invalid"} {
		if isLoopbackAddress(address) {
			t.Fatalf("%q should not be loopback", address)
		}
	}
}
