package http_config

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestKernelAppliesDefaultNamedAndFallbackMiddleware(t *testing.T) {
	for _, names := range [][]string{nil, {"hello"}, {"missing"}} {
		recorder := httptest.NewRecorder()
		KernelHandler(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) }, names...)(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
		if recorder.Code != http.StatusCreated || recorder.Header().Get("AKOFLOW-SERVER") == "" {
			t.Fatalf("default middleware failed for %v", names)
		}
		if len(names) == 1 && names[0] == "hello" && recorder.Header().Get("HELLO-MIDDLEWARE") == "" {
			t.Fatal("hello middleware missing")
		}
	}
	if len(Middlewares()) != 1 {
		t.Fatal("unexpected default middleware count")
	}
}
