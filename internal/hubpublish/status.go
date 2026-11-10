package hubpublish

import (
	"context"
	"time"
)

func (m *Manager) Status(ctx context.Context) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked(ctx, true)
}
func (m *Manager) statusLocked(ctx context.Context, verify bool) (Status, error) {
	result := Status{Enabled: m.state.Enabled, HubURL: m.state.HubURL, ReadOnly: m.options.ReadOnly, DefaultPublicationID: m.state.DefaultPublicationID}
	if m.state.DeviceCode != "" && time.Now().Before(m.state.DeviceExpiresAt) {
		result.UserCode = m.state.UserCode
		result.VerificationURL = m.state.VerificationURL
		result.ExpiresAt = &m.state.DeviceExpiresAt
	}
	if !verify || m.state.AccessToken == "" || !m.state.Enabled {
		return result, nil
	}
	identity, err := m.identityLocked(ctx)
	if err != nil {
		if isReconnect(err) {
			m.clearAuthorizationLocked()
			_ = m.persistLocked()
		}
		result.Message = err.Error()
		return result, nil
	}
	result.Identity = &identity
	result.Connected = true
	return result, nil
}
func (m *Manager) identityLocked(ctx context.Context) (Identity, error) {
	var identity Identity
	if err := m.enabledLocked(); err != nil {
		return identity, err
	}
	if m.state.AccessToken == "" {
		return identity, failure(401, "hub_reconnect", "Sign in to Hub from project settings before publishing.")
	}
	err := m.requestLocked(ctx, "GET", "/desktop/me", nil, nil, true, &identity)
	return identity, err
}
