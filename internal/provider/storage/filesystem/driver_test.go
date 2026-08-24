package filesystem

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestDriverRoundTripAndRootProtection(t *testing.T) {
	driver, err := New(domain.StoragePVC, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	location, err := driver.Put(context.Background(), ports.PutObjectRequest{
		Key: "run/activity/result.txt", Source: bytes.NewBufferString("result"),
	})
	if err != nil {
		t.Fatal(err)
	}
	stat, err := driver.Stat(context.Background(), location)
	if err != nil || stat.SizeBytes != 6 || stat.Checksum == "" {
		t.Fatalf("stat=%+v err=%v", stat, err)
	}
	var output bytes.Buffer
	if err := driver.Get(context.Background(), ports.GetObjectRequest{Location: location, Target: &output}); err != nil || output.String() != "result" {
		t.Fatalf("output=%q err=%v", output.String(), err)
	}
	if _, err := driver.Put(context.Background(), ports.PutObjectRequest{Key: "../escape", Source: bytes.NewReader(nil)}); err == nil {
		t.Fatal("path traversal must fail")
	}
	if err := driver.Delete(context.Background(), location); err != nil {
		t.Fatal(err)
	}
}

func TestBrowseIsPagedAndBlocksTraversalAndEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	d, err := New(domain.StorageLocal, root)
	if err != nil {
		t.Fatal(err)
	}
	s := domain.StorageResource{ID: "local", Endpoint: root, BrowseRoots: []domain.StorageBrowseRoot{{Path: root}}}
	page, err := d.Browse(context.Background(), s, domain.BrowseRequest{Limit: 1})
	if err != nil || len(page.Entries) != 1 || page.NextCursor == "" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if _, err = d.Browse(context.Background(), s, domain.BrowseRequest{Path: "../"}); err == nil {
		t.Fatal("traversal allowed")
	}
	if _, err = d.Browse(context.Background(), s, domain.BrowseRequest{Path: "outside"}); err == nil {
		t.Fatal("symlink escape allowed")
	}
}

func TestWriteCreatesAnExplicitlyConfiguredMissingRoot(t *testing.T) {
	driver, err := New(domain.StorageLocal, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "new-cache")
	storage := domain.StorageResource{
		ID: "cache", Endpoint: root,
		BrowseRoots: []domain.StorageBrowseRoot{{Path: root}},
	}
	if err := driver.Write(
		context.Background(), storage, "images/tool.sif",
		bytes.NewBufferString("sif bytes"), 9,
	); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "images", "tool.sif"))
	if err != nil || string(content) != "sif bytes" {
		t.Fatalf("content=%q err=%v", content, err)
	}
}
