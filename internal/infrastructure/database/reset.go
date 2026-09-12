package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const factoryResetSuffix = ".factory-reset-pending"

// ScheduleFactoryReset records a reset request without modifying the database
// while it is open. The marker is consumed before SQLite is opened again.
func ScheduleFactoryReset(path string) error {
	path, err := normalizePath(path)
	if err != nil {
		return err
	}
	marker := path + factoryResetSuffix
	temporary := marker + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	if err := os.WriteFile(temporary, []byte("factory-reset\n"), 0o600); err != nil {
		return fmt.Errorf("write factory reset marker: %w", err)
	}
	if err := os.Rename(temporary, marker); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish factory reset marker: %w", err)
	}
	return nil
}

// ApplyPendingFactoryReset removes the SQLite database and its sidecar files.
// It must run before any connection to path is opened. The marker is removed
// last so an interrupted reset is retried on the next process start.
func ApplyPendingFactoryReset(path string) (bool, error) {
	path, err := normalizePath(path)
	if err != nil {
		return false, err
	}
	marker := path + factoryResetSuffix
	if _, err := os.Stat(marker); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("inspect factory reset marker: %w", err)
	}

	for _, candidate := range []string{path, path + "-wal", path + "-shm", path + "-journal"} {
		if err := os.Remove(candidate); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("remove SQLite file %s: %w", filepath.Base(candidate), err)
		}
	}
	if err := os.Remove(marker); err != nil {
		return false, fmt.Errorf("remove factory reset marker: %w", err)
	}
	return true, nil
}
