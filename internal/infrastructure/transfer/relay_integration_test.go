package transfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	applicationtransfer "github.com/UFFeScience/akoflow/internal/application/transfer"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func kindTransferEndpoint() domain.TransferEndpoint {
	return domain.TransferEndpoint{
		URI: "kubernetes:///workspace?namespace=default&claim=akoflow-transfer-validation",
		Configuration: map[string]string{
			"server": "https://host.docker.internal:61640", "tokenFile": "/app/storage/credentials/kubernetes/kubernetes-environment-kubernetes-token.token",
			"insecureSkipTLSVerify": "true",
		},
	}
}

type integrationEndpointConnector struct {
	ports.TransferConnector
	endpoint domain.TransferEndpoint
}

func (connector integrationEndpointConnector) CanHandle(candidate domain.TransferEndpoint) bool {
	return connector.TransferConnector.CanHandle(candidate)
}

// Materializer intentionally persists only non-secret endpoint URIs. This
// adapter injects the credential configuration that production resolves from
// the environment connection store.
func (connector integrationEndpointConnector) endpointFor(candidate domain.TransferEndpoint) domain.TransferEndpoint {
	configured := connector.endpoint
	configured.URI = candidate.URI
	return configured
}
func (connector integrationEndpointConnector) Exists(ctx context.Context, endpoint domain.TransferEndpoint, name string) (bool, error) {
	return connector.TransferConnector.Exists(ctx, connector.endpointFor(endpoint), name)
}
func (connector integrationEndpointConnector) Open(ctx context.Context, endpoint domain.TransferEndpoint, name string, offset int64) (io.ReadCloser, error) {
	return connector.TransferConnector.Open(ctx, connector.endpointFor(endpoint), name, offset)
}
func (connector integrationEndpointConnector) Put(ctx context.Context, endpoint domain.TransferEndpoint, name string, input io.Reader, offset int64) error {
	return connector.TransferConnector.Put(ctx, connector.endpointFor(endpoint), name, input, offset)
}
func (connector integrationEndpointConnector) Commit(ctx context.Context, endpoint domain.TransferEndpoint, partial, final string) error {
	return connector.TransferConnector.Commit(ctx, connector.endpointFor(endpoint), partial, final)
}

func TestRelayPlafrimToKindAndBack(t *testing.T) {
	if os.Getenv("AKOFLOW_RELAY_INTEGRATION") == "" {
		t.Skip("set AKOFLOW_RELAY_INTEGRATION to run against Plafrim and Kind")
	}
	buffer := BufferSizeProvider(func(context.Context) int { return 64 << 10 })
	sshEndpoint, kubernetesEndpoint := plafrimTransferEndpoint(), kindTransferEndpoint()
	materializer := applicationtransfer.Materializer{Connectors: []ports.TransferConnector{
		integrationEndpointConnector{TransferConnector: RsyncSSH{BufferSize: buffer}, endpoint: sshEndpoint},
		integrationEndpointConnector{TransferConnector: KubernetesExec{BufferSize: buffer}, endpoint: kubernetesEndpoint},
	}}
	payload := bytes.Repeat([]byte("akoflow-hpc-stream-validation\n"), 4096)
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(payload))
	blob := domain.BlobDescriptor{Digest: digest, SizeBytes: int64(len(payload))}
	forward := domain.DataTransferPlan{ID: "plafrim-kind", Strategy: domain.TransferGateway,
		Source:      domain.TransferLocation{URI: sshEndpoint.URI, Path: "payload"},
		Destination: domain.TransferLocation{URI: kubernetesEndpoint.URI, Path: "plafrim-to-kind-v3"}, Blobs: []domain.BlobDescriptor{blob}}
	_, forwardRun, err := materializer.Materialize(context.Background(), forward, domain.ArtifactMaterialization{ID: "plafrim-kind", Digest: digest})
	if err != nil || forwardRun.Status != domain.TransferCompleted || len(forwardRun.VerifiedBlobs) != 1 {
		reader, openErr := (KubernetesExec{}).Open(context.Background(), kubernetesEndpoint, "plafrim-to-kind-v3/"+digest+".partial", 0)
		if openErr == nil {
			hash := sha256.New()
			count, readErr := io.Copy(hash, reader)
			closeErr := reader.Close()
			t.Logf("destination partial bytes=%d sha256=%x read=%v close=%v", count, hash.Sum(nil), readErr, closeErr)
		} else {
			t.Logf("destination partial unavailable: %v", openErr)
		}
		t.Fatalf("forward result=%+v error=%v", forwardRun, err)
	}
	reverse := domain.DataTransferPlan{ID: "kind-plafrim", Strategy: domain.TransferGateway,
		Source:      domain.TransferLocation{URI: kubernetesEndpoint.URI, Path: "plafrim-to-kind-v3/" + digest},
		Destination: domain.TransferLocation{URI: sshEndpoint.URI, Path: "kind-to-plafrim-v3"}, Blobs: []domain.BlobDescriptor{blob}}
	_, reverseRun, err := materializer.Materialize(context.Background(), reverse, domain.ArtifactMaterialization{ID: "kind-plafrim", Digest: digest})
	if err != nil || reverseRun.Status != domain.TransferCompleted || len(reverseRun.VerifiedBlobs) != 1 {
		t.Fatalf("reverse result=%+v error=%v", reverseRun, err)
	}
}
