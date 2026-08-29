package transfer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	domaininstance "github.com/UFFeScience/akoflow/internal/domain/instance"
)

type instanceStoreFake struct {
	value *domaininstance.Instance
	err   error
}

func (store instanceStoreFake) Find(context.Context) (*domaininstance.Instance, error) {
	return store.value, store.err
}
func (instanceStoreFake) Save(context.Context, domaininstance.Instance) error { return nil }
func (instanceStoreFake) FindPreferences(context.Context, string) (*domaininstance.UserPreferences, error) {
	return nil, nil
}
func (instanceStoreFake) SavePreferences(context.Context, domaininstance.UserPreferences) error {
	return nil
}

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

func TestNormalizeBufferSizeUsesConfiguredBounds(t *testing.T) {
	if got := normalizeBufferSize(domaininstance.MinTransferBufferBytes); got != int(domaininstance.MinTransferBufferBytes) {
		t.Fatalf("minimum buffer=%d", got)
	}
	for _, invalid := range []int64{0, domaininstance.MinTransferBufferBytes - 1, domaininstance.MaxTransferBufferBytes + 1} {
		if got := normalizeBufferSize(invalid); got != int(domaininstance.DefaultTransferBufferBytes) {
			t.Fatalf("invalid buffer %d normalized to %d", invalid, got)
		}
	}
}

func TestCopyWithBufferUsesProviderAndPreservesPayload(t *testing.T) {
	payload := bytes.Repeat([]byte("workspace"), 1024)
	var destination bytes.Buffer
	providerCalled := false
	written, err := copyWithBuffer(context.Background(), &destination, bytes.NewReader(payload), func(context.Context) int {
		providerCalled = true
		return 4096
	})
	if err != nil {
		t.Fatal(err)
	}
	if !providerCalled || written != int64(len(payload)) || !bytes.Equal(destination.Bytes(), payload) {
		t.Fatalf("copy result called=%v written=%d size=%d", providerCalled, written, destination.Len())
	}
}

func TestInstanceBufferSizeReadsConfigurationAndFallsBack(t *testing.T) {
	configured := InstanceBufferSize(instanceStoreFake{value: &domaininstance.Instance{
		TransferBufferBytes: domaininstance.MinTransferBufferBytes,
	}})
	if got := configured(context.Background()); got != int(domaininstance.MinTransferBufferBytes) {
		t.Fatalf("configured buffer=%d", got)
	}
	for _, store := range []instanceStoreFake{{}, {err: errors.New("database unavailable")}} {
		if got := InstanceBufferSize(store)(context.Background()); got != int(domaininstance.DefaultTransferBufferBytes) {
			t.Fatalf("fallback buffer=%d", got)
		}
	}
}
