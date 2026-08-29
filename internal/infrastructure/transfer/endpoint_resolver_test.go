package transfer

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type connectionStoreStub struct {
	connection domain.EnvironmentConnection
	err        error
	missing    bool
}

func (store connectionStoreStub) FindConnection(context.Context, string) (*domain.EnvironmentConnection, error) {
	if store.err != nil || store.missing {
		return nil, store.err
	}
	value := store.connection
	return &value, nil
}

func TestEnvironmentEndpointResolverWithoutConnection(t *testing.T) {
	resolver := EnvironmentEndpointResolver{}
	endpoint, err := resolver.ResolveTransferEndpoint(context.Background(), domain.TransferLocation{URI: "file:///tmp/data"})
	if err != nil || endpoint.URI != "file:///tmp/data" || endpoint.Configuration != nil {
		t.Fatalf("endpoint=%+v err=%v", endpoint, err)
	}
	if _, err := resolver.ResolveTransferEndpoint(context.Background(), domain.TransferLocation{URI: "ssh:///tmp?connectionId=ssh"}); err == nil {
		t.Fatal("connection store requirement expected")
	}
	if _, err := resolver.ResolveTransferEndpoint(context.Background(), domain.TransferLocation{URI: "://bad"}); err == nil {
		t.Fatal("malformed URI must fail")
	}
}

func TestEnvironmentEndpointResolverBuildsSSHConnection(t *testing.T) {
	connection := domain.EnvironmentConnection{
		ID: "ssh", EnvironmentID: "environment", Type: domain.ConnectionSSH,
		Endpoint: "login.example", Username: "researcher", CredentialRef: "file:/keys/id",
		Configuration: map[string]any{"port": float64(2222), "proxyCommand": "ssh jump", "forwardAgent": true},
	}
	resolver := EnvironmentEndpointResolver{Connections: connectionStoreStub{connection: connection}}
	endpoint, err := resolver.ResolveTransferEndpoint(context.Background(), domain.TransferLocation{URI: "file:///scratch/run?connectionId=ssh"})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"ssh://researcher@login.example/scratch/run", "port=2222", "proxyCommand=", "knownHostsFile=", "forwardAgent=true", "identityFile="} {
		if !strings.Contains(endpoint.URI, expected) {
			t.Fatalf("URI=%q missing %q", endpoint.URI, expected)
		}
	}
	connection.Configuration = map[string]any{"port": 22, "knownHostsFile": "/known"}
	endpoint, err = (EnvironmentEndpointResolver{Connections: connectionStoreStub{connection: connection}}).ResolveTransferEndpoint(context.Background(), domain.TransferLocation{URI: "file:///scratch?connectionId=ssh"})
	if err != nil || !strings.Contains(endpoint.URI, "port=22") || !strings.Contains(endpoint.URI, "%2Fknown") {
		t.Fatalf("endpoint=%+v err=%v", endpoint, err)
	}
}

func TestEnvironmentEndpointResolverReportsLookupAndTypeErrors(t *testing.T) {
	location := domain.TransferLocation{URI: "file:///data?connectionId=connection"}
	if _, err := (EnvironmentEndpointResolver{Connections: connectionStoreStub{err: errors.New("database")}}).ResolveTransferEndpoint(context.Background(), location); err == nil {
		t.Fatal("lookup error expected")
	}
	if _, err := (EnvironmentEndpointResolver{Connections: connectionStoreStub{missing: true}}).ResolveTransferEndpoint(context.Background(), location); err == nil {
		t.Fatal("missing connection error expected")
	}
	resolver := EnvironmentEndpointResolver{Connections: connectionStoreStub{connection: domain.EnvironmentConnection{ID: "connection", Type: domain.ConnectionLocal}}}
	if _, err := resolver.ResolveTransferEndpoint(context.Background(), location); err == nil {
		t.Fatal("unsupported connection type must fail")
	}
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
