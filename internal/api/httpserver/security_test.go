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
	request = httptest.NewRequest(http.MethodHead, "/akoflow-api/instance/", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("public instance HEAD got %d, want 204", response.Code)
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
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("root health GET got %d, want 401", response.Code)
	}
}

func TestSecureAPIRestrictsNonLoopbackRequests(t *testing.T) {
	handler := SecureAPI(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }), SecurityOptions{LoopbackOnly: true})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.10:4000"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("remote = %d", response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:4000"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("loopback = %d", response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "not-an-ip"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("invalid remote = %d", response.Code)
	}
}

func TestPublicBootstrapAndBearerParsing(t *testing.T) {
	for _, path := range []string{"/akoflow-api/instance", "/akoflow-api/instance/", "/akoflow-api/preflight/"} {
		if !isPublicBootstrapRequest(httptest.NewRequest(http.MethodGet, path, nil)) {
			t.Fatalf("%s should be public", path)
		}
		if !isPublicBootstrapRequest(httptest.NewRequest(http.MethodHead, path, nil)) {
			t.Fatalf("HEAD %s should be public", path)
		}
	}
	if isPublicBootstrapRequest(httptest.NewRequest(http.MethodPost, "/akoflow-api/preflight/", nil)) {
		t.Fatal("POST should not be public")
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer secret")
	if !validBearer(request, "secret") || validBearer(request, "other") {
		t.Fatal("bearer validation mismatch")
	}
}
