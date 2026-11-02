package mcpcontrol

import (
	"errors"
	"os"
	"path/filepath"
)

// StateDirectory keeps installation credentials separate from instance exports.
func StateDirectory(configured, databasePath string) string {
	if configured != "" {
		return configured
	}
	return filepath.Join(filepath.Dir(databasePath), "mcp")
}

// ScheduleReset revokes saved app connections when the daemon next starts.
// State is cleared before any MCP handler can accept requests.
func ScheduleReset(directory string) error {
	return privateWrite(filepath.Join(directory, ".factory-reset-pending"), []byte("reset\n"))
}

func (m *Manager) applyPendingReset() error {
	marker := filepath.Join(m.options.Directory, ".factory-reset-pending")
	if _, err := os.Stat(marker); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, name := range []string{"credentials", "sessions"} {
		if err := os.RemoveAll(filepath.Join(m.options.Directory, name)); err != nil {
			return err
		}
	}
	m.enabled = false
	if err := m.persistLocked(); err != nil {
		return err
	}
	return os.Remove(marker)
}
