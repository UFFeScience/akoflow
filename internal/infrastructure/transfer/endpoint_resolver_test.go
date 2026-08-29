package transfer

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type connectionStoreStub struct{ connection domain.EnvironmentConnection }

func (store connectionStoreStub) FindConnection(context.Context, string) (*domain.EnvironmentConnection, error) {
	value := store.connection
	return &value, nil
}
func (connectionStoreStub) ListAllConnections(context.Context) ([]domain.EnvironmentConnection, error) {
	return nil, nil
}
func (connectionStoreStub) SaveConnectionCheck(context.Context, domain.ConnectionCheck) error {
	return nil
}
func (connectionStoreStub) ListConnectionChecks(context.Context, string, int) ([]domain.ConnectionCheck, error) {
	return nil, nil
}

func TestEnvironmentEndpointResolverInjectsKubernetesCredential(t *testing.T) {
	resolver := EnvironmentEndpointResolver{Connections: connectionStoreStub{connection: domain.EnvironmentConnection{
		ID: "kind", EnvironmentID: "environment", Type: domain.ConnectionKubernetes,
		Endpoint: "https://kind.test", CredentialRef: "file:/credentials/kind.token",
		Configuration: map[string]any{"insecureSkipTlsVerify": true},
	}}}
	endpoint, err := resolver.ResolveTransferEndpoint(context.Background(), domain.TransferLocation{
		URI: "kubernetes://kind/workspace?namespace=default&claim=data",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !((KubernetesExec{}).CanHandle(endpoint)) || endpoint.Configuration["server"] != "https://kind.test" || endpoint.Configuration["tokenFile"] != "/credentials/kind.token" {
		t.Fatalf("unexpected resolved endpoint: %+v", endpoint)
	}
}
