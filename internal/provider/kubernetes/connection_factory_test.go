package kubernetes

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestConnectionFactoryBuildsIsolatedAdapterFromRuntimeConnection(t *testing.T) {
	t.Setenv("KIND_TOKEN", "token")
	adapter, err := (ConnectionFactory{DefaultNamespace: "default"}).Build(
		domain.EnvironmentRuntime{ID: "kind-runtime", Driver: domain.RuntimeDriverKubernetes,
			Configuration: map[string]any{"connectionId": "kind", "namespace": "science"}},
		domain.EnvironmentConnection{ID: "kind", Type: domain.ConnectionKubernetes,
			Endpoint: "https://kind.example:6443", CredentialRef: "env:KIND_TOKEN",
			Configuration: map[string]any{"insecureSkipTlsVerify": true}},
	)
	if err != nil {
		t.Fatal(err)
	}
	configured, ok := adapter.(*Adapter)
	if !ok || configured.namespace != "science" || configured.api == nil {
		t.Fatalf("adapter=%#v", adapter)
	}
}

func TestConnectionFactoryRejectsMissingCredential(t *testing.T) {
	_, err := (ConnectionFactory{}).Build(domain.EnvironmentRuntime{}, domain.EnvironmentConnection{
		ID: "kind", Type: domain.ConnectionKubernetes, Endpoint: "https://kind.example", CredentialRef: "env:MISSING_TOKEN",
	})
	if err == nil {
		t.Fatal("expected missing credential error")
	}
}
