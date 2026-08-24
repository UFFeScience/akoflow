package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
