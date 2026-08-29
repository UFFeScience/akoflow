package transfer

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"os"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func plafrimTransferEndpoint() domain.TransferEndpoint {
	query := url.Values{
		"identityFile":   {"/app/storage/credentials/ssh/plafrim-personal"},
		"knownHostsFile": {"/app/storage/credentials/ssh/known_hosts"},
		"proxyCommand":   {"ssh -A -l wferreir ssh.plafrim.fr -W plafrim:22"},
	}
	return domain.TransferEndpoint{URI: "ssh://wferreir@plafrim/home/wferreir/.akoflow/transfer-validation?" + query.Encode()}
}

func TestRsyncSSHStreamingRoundTrip(t *testing.T) {
	if os.Getenv("AKOFLOW_HPC_TRANSFER_INTEGRATION") == "" {
		t.Skip("set AKOFLOW_HPC_TRANSFER_INTEGRATION to run against Plafrim")
	}
	endpoint := plafrimTransferEndpoint()
	payload := bytes.Repeat([]byte("akoflow-hpc-stream-validation\n"), 4096)
	connector := RsyncSSH{BufferSize: func(context.Context) int { return 64 << 10 }}
	ctx := context.Background()
	if err := connector.Put(ctx, endpoint, "payload.partial", bytes.NewReader(payload), 0); err != nil {
		t.Fatal(err)
	}
	if err := connector.Commit(ctx, endpoint, "payload.partial", "payload"); err != nil {
		t.Fatal(err)
	}
	reader, err := connector.Open(ctx, endpoint, "payload", 0)
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
