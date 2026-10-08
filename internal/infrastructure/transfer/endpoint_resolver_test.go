package transfer

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type verifiedCloudStore struct {
	ports.CloudConfigurationStore
	instance domain.CloudProvisionedInstance
}

func (s verifiedCloudStore) ListProvisionedInstances(context.Context, string) ([]domain.CloudProvisionedInstance, error) {
	return []domain.CloudProvisionedInstance{s.instance}, nil
}

func TestCloudWorkspaceEndpointUsesVerifiedGenerationTrust(t *testing.T) {
	instance := domain.CloudProvisionedInstance{ID: "vm", Status: "ready", CapacityTargetID: "target", ProviderID: "generation", PublicAddress: "cloud.example", TerraformOutput: map[string]any{
		"verifiedHostGeneration": "generation", "verifiedHostKeyAlias": "verified-alias", "verifiedKnownHostsFile": "/verified/known_hosts",
	}}
	connection := domain.EnvironmentConnection{ID: "cloud", Type: domain.ConnectionCloud}
	location := domain.TransferLocation{URI: "file:///workspace?connectionId=cloud", ResourceID: "target", CloudInstanceID: "vm",
		WorkspaceSnapshotID: "sealed-manifest", WorkspaceSnapshotEntries: []domain.WorkspaceEntry{{Path: "input.fits", SizeBytes: 10, Digest: "sha256:content"}}}
	resolve := func() (domain.TransferEndpoint, error) {
		return (EnvironmentEndpointResolver{Connections: connectionStoreStub{connection: connection}, Cloud: verifiedCloudStore{instance: instance}}).ResolveTransferEndpoint(context.Background(), location)
	}
	endpoint, err := resolve()
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.WorkspaceSnapshotID != location.WorkspaceSnapshotID || len(endpoint.WorkspaceSnapshotEntries) != 1 {
		t.Fatal("generation resolution discarded sealed snapshot identity")
	}
	endpoint.WorkspaceSnapshotEntries[0].Path = "changed"
	if location.WorkspaceSnapshotEntries[0].Path != "input.fits" {
		t.Fatal("resolved endpoint aliases producer manifest entries")
	}
	args := strings.Join(sshArgs(endpoint), " ")
	for _, want := range []string{"StrictHostKeyChecking=yes", "HostKeyAlgorithms=ssh-ed25519", "HostKeyAlias=verified-alias", "UserKnownHostsFile=/verified/known_hosts"} {
		if !strings.Contains(args, want) {
			t.Fatalf("missing %s in %s", want, args)
		}
	}
	if strings.Contains(args, "accept-new") {
		t.Fatal("must not enroll via SSH")
	}
	instance.ProviderID = "replaced"
	if _, err := resolve(); err == nil {
		t.Fatal("stale generation accepted")
	}
	instance.TerraformOutput = nil
	if _, err := resolve(); err == nil {
		t.Fatal("missing verification accepted")
	}
}

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
	resolver := EnvironmentEndpointResolver{Connections: connectionStoreStub{connection: domain.EnvironmentConnection{ID: "connection", Type: domain.ConnectionType("unsupported")}}}
	if _, err := resolver.ResolveTransferEndpoint(context.Background(), location); err == nil {
		t.Fatal("unsupported connection type must fail")
	}
}

func TestEnvironmentEndpointResolverKeepsLocalFileEndpoint(t *testing.T) {
	connection := domain.EnvironmentConnection{
		ID: "local-environment-local-connection", EnvironmentID: "local-environment",
		Type: domain.ConnectionLocal,
	}
	resolver := EnvironmentEndpointResolver{Connections: connectionStoreStub{connection: connection}}
	endpoint, err := resolver.ResolveTransferEndpoint(context.Background(), domain.TransferLocation{
		URI: "file:///tmp/akoflow/run/workflow-a?connectionId=local-environment-local-connection",
	})
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.URI != "file:///tmp/akoflow/run/workflow-a" {
		t.Fatalf("unexpected local endpoint URI: %q", endpoint.URI)
	}
	if endpoint.ConnectionID != connection.ID || endpoint.EnvironmentID != connection.EnvironmentID {
		t.Fatalf("unexpected local endpoint metadata: %+v", endpoint)
	}
	if _, err := resolver.ResolveTransferEndpoint(context.Background(), domain.TransferLocation{
		URI: "ssh:///tmp/akoflow/run/workflow-a?connectionId=local-environment-local-connection",
	}); err == nil {
		t.Fatal("local connection must reject non-file URI schemes")
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
	if !((&KubernetesExec{}).CanHandle(endpoint)) || endpoint.Configuration["server"] != "https://kind.test" || endpoint.Configuration["tokenFile"] != "/credentials/kind.token" {
		t.Fatalf("unexpected resolved endpoint: %+v", endpoint)
	}
}
