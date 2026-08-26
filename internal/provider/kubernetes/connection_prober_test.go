package kubernetes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestConnectionProberUsesConfiguredNamespace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer saved-token" {
			t.Errorf("authorization=%q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/api/v1/namespaces/akoflow/pods" {
			t.Errorf("path=%q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()
	health := NewConnectionProber("default").Probe(
		context.Background(),
		domain.EnvironmentConnection{Endpoint: server.URL, Configuration: map[string]any{"namespace": "akoflow", "bearerToken": "saved-token"}},
	)
	if !health.Healthy || !strings.Contains(health.Message, "akoflow") {
		t.Fatalf("health=%+v", health)
	}
}

func TestConnectionProberReportsMissingConnectionSettings(t *testing.T) {
	health := NewConnectionProber("akoflow").Probe(context.Background(), domain.EnvironmentConnection{})
	if health.Healthy || !strings.Contains(health.Message, "invalid") {
		t.Fatalf("health=%+v", health)
	}
}
