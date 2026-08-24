package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecureAPIRequiresBearerToken(t *testing.T) {
	handler := SecureAPI(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), SecurityOptions{BearerToken: "test-token"})
	request := httptest.NewRequest(http.MethodGet, "/akoflow-api/storages/x/entries/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", response.Code)
	}
	request.Header.Set("Authorization", "Bearer test-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", response.Code)
	}
}

func TestSecureAPIAllowsOnlyPublicInstanceBootstrapWithoutToken(t *testing.T) {
	handler := SecureAPI(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), SecurityOptions{BearerToken: "test-token"})

	request := httptest.NewRequest(http.MethodGet, "/akoflow-api/instance/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("public instance GET got %d, want 204", response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/akoflow-api/instance", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("public instance GET without trailing slash got %d, want 204", response.Code)
	}

	request = httptest.NewRequest(http.MethodPut, "/akoflow-api/instance/", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("instance PUT got %d, want 401", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/akoflow-api/environments/", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("environment GET got %d, want 401", response.Code)
	}
}
