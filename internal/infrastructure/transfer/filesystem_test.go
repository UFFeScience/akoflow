package transfer

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestLocalFilesystemLifecycleAndResume(t *testing.T) {
	root := t.TempDir()
	endpoint := domain.TransferEndpoint{URI: "file://" + root}
	connector := LocalFilesystem{BufferSize: func(context.Context) int { return 4096 }}
	if !connector.CanHandle(endpoint) || connector.CanHandle(domain.TransferEndpoint{URI: "https://example.test"}) {
		t.Fatal("unexpected connector support")
	}
	if err := connector.Put(context.Background(), endpoint, "nested/file.partial", bytes.NewBufferString("hello"), 0); err != nil {
		t.Fatal(err)
	}
	if err := connector.Put(context.Background(), endpoint, "nested/file.partial", bytes.NewBufferString(" world"), 5); err != nil {
		t.Fatal(err)
	}
	exists, err := connector.Exists(context.Background(), endpoint, "nested/file.partial")
	if err != nil || !exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	reader, err := connector.Open(context.Background(), endpoint, "nested/file.partial", 6)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || string(payload) != "world" {
		t.Fatalf("payload=%q err=%v", payload, err)
	}
	if err := connector.Commit(context.Background(), endpoint, "nested/file.partial", "nested/file.txt"); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(root, "nested", "file.txt"))
	if err != nil || string(stored) != "hello world" {
		t.Fatalf("stored=%q err=%v", stored, err)
	}
}

func TestLocalFilesystemRejectsTraversalAndSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	endpoint := domain.TransferEndpoint{URI: "file://" + root}
	if _, err := localPath(endpoint, "../escape"); err == nil {
		t.Fatal("path traversal must fail")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := localPath(endpoint, "link/file"); err == nil {
		t.Fatal("symlink escape must fail")
	}
}
