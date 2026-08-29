package transfer

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestArtifactStoreReadsConfinedBuildOutput(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sha256", "digest")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("artifact"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := ArtifactStore{Root: root}
	endpoint := domain.TransferEndpoint{URI: "artifact://sha256/digest"}
	if !store.CanHandle(endpoint) {
		t.Fatal("artifact endpoint was not recognized")
	}
	exists, err := store.Exists(context.Background(), endpoint, "")
	if err != nil || !exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	reader, err := store.Open(context.Background(), endpoint, "", 3)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || string(payload) != "ifact" {
		t.Fatalf("payload=%q err=%v", payload, err)
	}
	if err := store.Put(context.Background(), endpoint, "", nil, 0); err == nil {
		t.Fatal("artifact store must be read-only")
	}
	if err := store.Commit(context.Background(), endpoint, "a", "b"); err == nil {
		t.Fatal("artifact store must be read-only")
	}
}

func TestArtifactStoreRejectsInvalidConfiguration(t *testing.T) {
	for _, store := range []ArtifactStore{{}, {Root: t.TempDir()}} {
		endpoint := domain.TransferEndpoint{URI: "file:///tmp/value"}
		if _, err := store.file(endpoint); err == nil {
			t.Fatalf("store=%+v accepted invalid endpoint", store)
		}
	}
}
