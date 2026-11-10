package hubpublish

import (
	"context"
	"strings"
)

func (m *Manager) SaveSettings(ctx context.Context, enabled bool, hubURL string) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.options.ReadOnly {
		return Status{}, failure(403, "read_only", "Imported snapshots cannot change Hub settings.")
	}
	origin, err := validateOrigin(hubURL)
	if err != nil {
		return Status{}, err
	}
	if origin != m.state.HubURL || !enabled {
		if m.state.AccessToken != "" {
			_ = m.requestLocked(ctx, "DELETE", "/desktop/session", nil, nil, true, nil)
		}
		m.clearAuthorizationLocked()
	}
	m.state.HubURL = origin
	m.state.Enabled = enabled
	if err = m.persistLocked(); err != nil {
		return Status{}, err
	}
	return m.statusLocked(ctx, true)
}
func (m *Manager) Disconnect(ctx context.Context) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.options.ReadOnly {
		return Status{}, failure(403, "read_only", "Imported snapshots cannot change Hub settings.")
	}
	if m.state.AccessToken != "" {
		_ = m.requestLocked(ctx, "DELETE", "/desktop/session", nil, nil, true, nil)
	}
	m.clearAuthorizationLocked()
	if err := m.persistLocked(); err != nil {
		return Status{}, err
	}
	return m.statusLocked(ctx, false)
}
func (m *Manager) clearAuthorizationLocked() {
	m.state.AccessToken = ""
	m.clearDeviceLocked()
	m.previews = map[string]preparedPublication{}
}
func (m *Manager) clearDeviceLocked() {
	m.state.DeviceCode = ""
	m.state.UserCode = ""
	m.state.VerificationURL = ""
}
func (m *Manager) enabledLocked() error {
	if m.options.ReadOnly {
		return failure(403, "read_only", "Imported snapshots cannot publish to Hub.")
	}
	if !m.state.Enabled || strings.TrimSpace(m.state.HubURL) == "" {
		return failure(409, "hub_disabled", "Enable AkôFlow Hub in project settings before publishing.")
	}
	return nil
}
