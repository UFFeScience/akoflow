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
	if hubURL != "" && strings.TrimRight(strings.TrimSpace(hubURL), "/") != m.options.HubURL {
		return Status{}, failure(422, "hub_managed", "This installation uses one AkôFlow Hub. Its service address is managed by the administrator.")
	}
	if enabled && m.state.HubURL == "" {
		return Status{}, failure(503, "hub_not_configured", "AkôFlow Hub is not configured for this installation. Contact the administrator.")
	}
	if !enabled {
		if m.state.AccessToken != "" {
			_ = m.requestLocked(ctx, "DELETE", "/desktop/session", nil, nil, true, nil)
		}
		m.clearAuthorizationLocked()
	}
	m.state.Enabled = enabled
	if err := m.persistLocked(); err != nil {
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
		return failure(409, "hub_disabled", "Sign in to AkôFlow Hub from project settings before publishing.")
	}
	return nil
}
