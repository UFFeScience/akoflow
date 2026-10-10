package sshfilesystem

import (
	"context"
	"errors"
	"testing"
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
