package transfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestKubernetesExecRoundTrip(t *testing.T) {
	if os.Getenv("AKOFLOW_K8_TRANSFER_INTEGRATION") == "" {
		t.Skip("set AKOFLOW_K8_TRANSFER_INTEGRATION to run against a real cluster")
	}
	endpoint := domain.TransferEndpoint{
		URI: "kubernetes:///workspace?namespace=default&claim=akoflow-transfer-validation",
		Configuration: map[string]string{
			"server": "https://host.docker.internal:61640", "tokenFile": "/app/storage/credentials/kubernetes/kubernetes-environment-kubernetes-token.token",
			"insecureSkipTLSVerify": "true",
		},
	}
	payload := bytes.Repeat([]byte("akoflow-stream-validation\n"), 4096)
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	connector := KubernetesExec{BufferSize: func(context.Context) int { return 64 << 10 }}
	ctx := context.Background()
	if err := connector.Put(ctx, endpoint, digest+".partial", bytes.NewReader(payload), 0); err != nil {
		t.Fatal(err)
	}
	if err := connector.Commit(ctx, endpoint, digest+".partial", digest); err != nil {
		t.Fatal(err)
	}
	reader, err := connector.Open(ctx, endpoint, digest, 0)
	if err != nil {
		t.Fatal(err)
	}
	contents, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read=%v close=%v", readErr, closeErr)
	}
	if !bytes.Equal(contents, payload) {
		t.Fatalf("round trip mismatch: got %d bytes, want %d", len(contents), len(payload))
	}
}
