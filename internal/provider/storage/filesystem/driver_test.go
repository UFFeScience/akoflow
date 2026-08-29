package filesystem

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

func TestBrowserStatOpenRemoveAndEntryTypes(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("file.txt", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	driver, err := New(domain.StorageNFS, root)
	if err != nil {
		t.Fatal(err)
	}
	storage := domain.StorageResource{ID: "storage", Endpoint: root, BrowseRoots: []domain.StorageBrowseRoot{{Path: root}}}
	page, err := driver.Browse(context.Background(), storage, domain.BrowseRequest{Cursor: "1", Limit: 500})
	if err != nil || len(page.Entries) != 2 {
		t.Fatalf("Browse()=%#v,%v", page, err)
	}
	file, err := driver.BrowseStat(context.Background(), storage, "file.txt")
	if err != nil || file.Type != domain.FileEntryFile || file.SizeBytes != 7 {
		t.Fatalf("file=%#v,%v", file, err)
	}
	directory, err := driver.BrowseStat(context.Background(), storage, "directory")
	if err != nil || directory.Type != domain.FileEntryDirectory {
		t.Fatalf("directory=%#v,%v", directory, err)
	}
	link, err := driver.BrowseStat(context.Background(), storage, "link")
	if err != nil || link.Type != domain.FileEntrySymlink || link.LinkTarget != "file.txt" {
		t.Fatalf("link=%#v,%v", link, err)
	}
	body, entry, err := driver.Open(context.Background(), storage, "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(body)
	_ = body.Close()
	if string(content) != "content" || entry.Name != "file.txt" {
		t.Fatalf("open=%q %#v", content, entry)
	}
	if _, _, err = driver.Open(context.Background(), storage, "directory"); err == nil {
		t.Fatal("expected directory open error")
	}
	if err = driver.Remove(context.Background(), storage, "directory"); err == nil {
		t.Fatal("expected directory removal error")
	}
	readOnly := storage
	readOnly.ReadOnly = true
	if err = driver.Remove(context.Background(), readOnly, "file.txt"); err == nil {
		t.Fatal("expected readonly removal error")
	}
	if err = driver.Write(context.Background(), readOnly, "new", bytes.NewReader(nil), 0); err == nil {
		t.Fatal("expected readonly write error")
	}
	if err = driver.Remove(context.Background(), storage, "file.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestDriverConfigurationAndInputValidation(t *testing.T) {
	if _, err := New(domain.StorageS3, t.TempDir()); err == nil {
		t.Fatal("expected unsupported type")
	}
	if _, err := New(domain.StorageLocal, ""); err == nil {
		t.Fatal("expected root error")
	}
	root := t.TempDir()
	driver, err := New(domain.StorageLustre, root)
	if err != nil {
		t.Fatal(err)
	}
	if driver.Type() != domain.StorageLustre {
		t.Fatalf("type=%s", driver.Type())
	}
	storage := domain.StorageResource{Endpoint: root, BrowseRoots: []domain.StorageBrowseRoot{{Path: t.TempDir()}}}
	if _, err = driver.Browse(context.Background(), storage, domain.BrowseRequest{}); err == nil {
		t.Fatal("expected disallowed root")
	}
	storage = domain.StorageResource{Endpoint: root, BrowseRoots: []domain.StorageBrowseRoot{{Path: root}}}
	if _, err = driver.Browse(context.Background(), storage, domain.BrowseRequest{Cursor: "bad"}); err == nil {
		t.Fatal("expected cursor error")
	}
	if _, err = driver.Stat(context.Background(), domain.DataLocation{URI: "https://example/file"}); err == nil {
		t.Fatal("expected file URI")
	}
	if err = driver.Get(context.Background(), ports.GetObjectRequest{Location: domain.DataLocation{URI: "invalid"}, Target: io.Discard}); err == nil {
		t.Fatal("expected invalid get")
	}
	if err = driver.Delete(context.Background(), domain.DataLocation{URI: "file:///outside"}); err == nil {
		t.Fatal("expected outside delete")
	}
	first := fmt.Errorf("first")
	if firstError(nil, first) != first || firstError(nil) != nil {
		t.Fatal("firstError mismatch")
	}
}
