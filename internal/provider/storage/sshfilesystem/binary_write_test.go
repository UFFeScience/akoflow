package sshfilesystem

import (
	"bytes"
	"context"
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
	// Exercise the actual shell transport independently of GNU realpath,
	// which is required by the Linux worker's filesystem scripts.
	got, err := driver.runInput(context.Background(), storage, "cat", want, root, "binary.tar.gz")
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("binary payload mismatch: length=%d error=%v", len(got), err)
	}
}
