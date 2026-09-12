package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPendingFactoryResetRemovesDatabaseAndSidecars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.db")
	for _, candidate := range []string{path, path + "-wal", path + "-shm", path + "-journal"} {
		if err := os.WriteFile(candidate, []byte("persisted data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := ScheduleFactoryReset(path); err != nil {
		t.Fatal(err)
	}

	applied, err := ApplyPendingFactoryReset(path)
	if err != nil {
		t.Fatal(err)
	}
	if !applied {
		t.Fatal("factory reset marker was not applied")
	}
	for _, candidate := range []string{
		path,
		path + "-wal",
		path + "-shm",
		path + "-journal",
		path + factoryResetSuffix,
	} {
		if _, err := os.Stat(candidate); !os.IsNotExist(err) {
			t.Fatalf("%s still exists after reset", filepath.Base(candidate))
		}
	}
}

func TestApplyPendingFactoryResetIsNoOpWithoutMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.db")
	if err := os.WriteFile(path, []byte("persisted data"), 0o600); err != nil {
		t.Fatal(err)
	}

	applied, err := ApplyPendingFactoryReset(path)
	if err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("factory reset unexpectedly applied")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database was modified without a reset marker: %v", err)
	}
}
