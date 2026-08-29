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

func TestTerminalRunnerStartsAndCleansInteractivePod(t *testing.T) {
	created, deleted := false, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			created = true
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"status":{"Phase":"Running"}}`))
		case http.MethodDelete:
			deleted = true
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()
	directory := t.TempDir()
	kubectl := filepath.Join(directory, "kubectl")
	if err := os.WriteFile(kubectl, []byte("#!/bin/sh\ncat\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	terminal, err := (TerminalRunner{}).StartInteractive(context.Background(), domain.EnvironmentConnection{
		Endpoint: server.URL, Configuration: map[string]any{"bearerToken": "token", "namespace": "science", "insecureSkipTlsVerify": true},
	}, domain.Resource{ProviderID: "https://api.example", Metadata: map[string]any{"observedHostname": "kind-worker"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := terminal.Write([]byte("exit\n")); err != nil {
		t.Fatal(err)
	}
	_ = terminal.Resize(24, 80)
	_ = terminal.Close()
	if !created || !deleted {
		t.Fatalf("created=%v deleted=%v", created, deleted)
	}
}

func TestTerminalRunnerValidatesConfigurationAndPodOutcome(t *testing.T) {
	if _, err := (TerminalRunner{}).StartInteractive(context.Background(), domain.EnvironmentConnection{CredentialRef: "env:"}, domain.Resource{}); err == nil || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("credential error=%v", err)
	}
	if _, err := (TerminalRunner{}).StartInteractive(context.Background(), domain.EnvironmentConnection{}, domain.Resource{}); err == nil {
		t.Fatal("missing endpoint and token must fail")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":{"Phase":"Failed"}}`))
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{Endpoint: server.URL, Token: "token", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if err := waitForPod(context.Background(), client, "default", "pod"); err == nil || !strings.Contains(err.Error(), "ended as Failed") {
		t.Fatalf("pod outcome error=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForPod(ctx, client, "default", "pod"); err == nil {
		t.Fatal("cancelled wait must fail")
	}
}
