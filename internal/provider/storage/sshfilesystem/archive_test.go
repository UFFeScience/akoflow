package sshfilesystem

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/provider"
)

func TestNativeArchivePublishesOnlyAfterRemoteValidation(t *testing.T) {
	executor := &commandFake{output: []byte{}}
	driver := New(connectionStoreFake{connection: testConnection()}, executor)
	storage := testStorage()
	result, err := driver.ArchiveDirectory(context.Background(), storage, "runs/run")
	if err != nil || result != "runs/run.tar.gz" || executor.calls != 1 {
		t.Fatalf("archive=%q calls=%d error=%v", result, executor.calls, err)
	}
	executor.err = errors.New("archive integrity check failed")
	if result, err := driver.ArchiveDirectory(context.Background(), storage, "runs/run"); err == nil || result != "" {
		t.Fatal("failed archive must not be published")
	}
	for _, value := range []string{"", "..", "../outside"} {
		if _, err := driver.ArchiveDirectory(context.Background(), storage, value); err == nil {
			t.Fatalf("accepted archive path %q", value)
		}
	}
	storage.ReadOnly = true
	if _, err := driver.ArchiveDirectory(context.Background(), storage, "runs/run"); err == nil {
		t.Fatal("read-only archive accepted")
	}
}

func TestNativeArchiveRealLinuxFilesystem(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("worker filesystem scripts require GNU realpath/find")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "runs", "run"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "runs", "run", "output.txt"), []byte("preserved output"), 0600); err != nil {
		t.Fatal(err)
	}
	driver := New(connectionStoreFake{connection: &domain.EnvironmentConnection{Type: domain.ConnectionLocal}}, provider.OSCommandExecutor{})
	storage := domain.StorageResource{Endpoint: root, Metadata: map[string]any{"connectionId": "local"}}
	archive, err := driver.ArchiveDirectory(context.Background(), storage, "runs/run")
	if err != nil {
		t.Fatal(err)
	}
	output, err := (provider.OSCommandExecutor{}).Run(context.Background(), "tar", []string{"-xOzf", filepath.Join(root, archive), "runs/run/output.txt"}, nil)
	if err != nil || string(output) != "preserved output" {
		t.Fatalf("archive contents=%q error=%v", output, err)
	}
}
