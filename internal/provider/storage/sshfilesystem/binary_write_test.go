package sshfilesystem

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/provider"
)

func TestRealShellWritePreservesBinaryStdin(t *testing.T) {
	root := t.TempDir()
	storage := domain.StorageResource{ID: "local", Type: domain.StorageSSH, Endpoint: root,
		Metadata: map[string]any{"connectionId": "local"}}
	driver := New(connectionStoreFake{connection: &domain.EnvironmentConnection{Type: domain.ConnectionLocal}}, provider.OSCommandExecutor{})
	want := []byte{0, 1, 255, 10, 13, 0, 42}
	if err := driver.Write(context.Background(), storage, "binary.tar.gz", bytes.NewReader(want), int64(len(want))); err != nil {
		t.Fatal(err)
	}
	reader, entry, err := driver.Open(context.Background(), storage, "binary.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(got, want) || entry.SizeBytes != int64(len(want)) {
		t.Fatalf("binary payload mismatch: length=%d entry=%d error=%v", len(got), entry.SizeBytes, err)
	}
}
