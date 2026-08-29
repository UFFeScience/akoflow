package kubernetes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestConnectionTokenSourcesAndErrors(t *testing.T) {
	t.Setenv("KUBERNETES_TEST_TOKEN", "environment-token")
	token, err := connectionToken(domain.EnvironmentConnection{CredentialRef: "env:KUBERNETES_TEST_TOKEN"})
	if err != nil || token != "environment-token" {
		t.Fatalf("token=%q err=%v", token, err)
	}
	for _, reference := range []string{"env:", "env:MISSING_KUBERNETES_TEST_TOKEN", "file:"} {
		if _, err := connectionToken(domain.EnvironmentConnection{CredentialRef: reference}); err == nil {
			t.Fatalf("invalid reference accepted: %q", reference)
		}
	}
	file := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(file, []byte(" file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	token, err = connectionToken(domain.EnvironmentConnection{CredentialRef: "file:" + file})
	if err != nil || token != "file-token" {
		t.Fatalf("token=%q err=%v", token, err)
	}
	if _, err := connectionToken(domain.EnvironmentConnection{CredentialRef: "file:" + file + ".missing"}); err == nil {
		t.Fatal("missing token file must fail")
	}
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := connectionToken(domain.EnvironmentConnection{CredentialRef: "file:" + empty}); err == nil {
		t.Fatal("empty token file must fail")
	}
}
