package transfer

import (
	"bytes"
	"io"
	"testing"
)

func TestBoundedRelayPreservesPayloadAcrossSmallChunks(t *testing.T) {
	payload := bytes.Repeat([]byte("relay"), 10000)
	relay := newBoundedRelay(bytes.NewReader(payload), 64<<10)
	defer relay.Close()
	result, err := io.ReadAll(relay)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, payload) {
		t.Fatalf("relay changed payload: got %d bytes, want %d", len(result), len(payload))
	}
}
