package kubernetes

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestConnectionFactoryBuildsIsolatedAdapterFromRuntimeConnection(t *testing.T) {
	adapter, err := (ConnectionFactory{DefaultNamespace: "default"}).Build(
		domain.EnvironmentRuntime{ID: "kind-runtime", Driver: domain.RuntimeDriverKubernetes,
			Configuration: map[string]any{"connectionId": "kind", "namespace": "science"}},
		domain.EnvironmentConnection{ID: "kind", Type: domain.ConnectionKubernetes,
			Endpoint: "https://kind.example:6443",
			Configuration: map[string]any{"bearerToken": "token", "insecureSkipTlsVerify": true}},
	)
	if err != nil {
		t.Fatal(err)
	}
	configured, ok := adapter.(*Adapter)
	if !ok || configured.namespace != "science" || configured.api == nil {
		t.Fatalf("adapter=%#v", adapter)
	}
	client, ok := configured.api.(*Client)
	if !ok || client.token != "token" {
		t.Fatalf("client=%#v", configured.api)
	}
}

func TestConnectionFactoryRejectsMissingCredential(t *testing.T) {
	_, err := (ConnectionFactory{}).Build(domain.EnvironmentRuntime{}, domain.EnvironmentConnection{
		ID: "kind", Type: domain.ConnectionKubernetes, Endpoint: "https://kind.example",
	})
	if err == nil {
		t.Fatal("expected missing credential error")
	}
}
