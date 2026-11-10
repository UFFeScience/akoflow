package hubpublish

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

func ScheduleReset(directory string) error {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, ".factory-reset-pending"), []byte("reset\n"), 0600)
}
func (m *Manager) applyPendingReset() error {
	marker := filepath.Join(m.options.Directory, ".factory-reset-pending")
	if _, err := os.Stat(marker); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if m.state.AccessToken != "" {
		_ = m.requestLocked(ctx, "DELETE", "/desktop/session", nil, nil, true, nil)
	}
	m.clearAuthorizationLocked()
	m.state.Enabled = false
	if err := m.persistLocked(); err != nil {
		return err
	}
	return os.Remove(marker)
}
