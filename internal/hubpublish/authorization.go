package hubpublish

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
)

func (m *Manager) StartAuthorization(ctx context.Context) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.options.ReadOnly {
		return Status{}, failure(403, "read_only", "Imported snapshots cannot connect to Hub.")
	}
	if m.state.HubURL == "" {
		return Status{}, failure(503, "hub_not_configured", "AkôFlow Hub is not configured for this installation. Contact the administrator.")
	}
	m.state.Enabled = true
	var instance struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := m.readSource(ctx, "/instance/", &instance); err != nil {
		return Status{}, err
	}
	name := instance.Name
	if name == "" {
		name = instance.ID
	}
	if name == "" {
		name = "AkôFlow project"
	}
	var result struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri_complete"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
	form := url.Values{"client_id": {"akoflow-desktop"}, "project_name": {name}, "scope": {"publications:write"}}
	if err := m.requestLocked(ctx, "POST", "/oauth/device/authorize", nil, form, false, &result); err != nil {
		return Status{}, err
	}
	if len(result.DeviceCode) != 64 || result.UserCode == "" || result.ExpiresIn <= 0 || result.ExpiresIn > 600 || !sameOrigin(result.VerificationURI, m.state.HubURL) {
		return Status{}, failure(502, "hub_response", "Hub returned an invalid authorization request.")
	}
	if m.state.AccessToken != "" {
		_ = m.requestLocked(ctx, "DELETE", "/desktop/session", nil, nil, true, nil)
	}
	m.clearAuthorizationLocked()
	m.state.DeviceCode = result.DeviceCode
	m.state.UserCode = result.UserCode
	m.state.VerificationURL = result.VerificationURI
	m.state.DeviceExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	m.state.PollInterval = max(5, result.Interval)
	m.state.NextPollAt = time.Now().Add(time.Duration(m.state.PollInterval) * time.Second)
	if err := m.persistLocked(); err != nil {
		return Status{}, err
	}
	return m.statusLocked(ctx, false)
}
func sameOrigin(value, origin string) bool {
	parsed, err := url.Parse(value)
	base, baseErr := url.Parse(origin)
	return err == nil && baseErr == nil && parsed.User == nil && parsed.Scheme == base.Scheme && strings.EqualFold(parsed.Host, base.Host)
}
func (m *Manager) PollAuthorization(ctx context.Context) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.enabledLocked(); err != nil {
		return Status{}, err
	}
	if m.state.DeviceCode == "" {
		return m.statusLocked(ctx, true)
	}
	if time.Now().After(m.state.DeviceExpiresAt) {
		m.clearDeviceLocked()
		_ = m.persistLocked()
		return Status{}, failure(409, "hub_expired", "The sign-in request expired. Start a new connection.")
	}
	if time.Now().Before(m.state.NextPollAt) {
		return m.statusLocked(ctx, false)
	}
	form := url.Values{"client_id": {"akoflow-desktop"}, "device_code": {m.state.DeviceCode}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}
	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
	}
	err := m.requestLocked(ctx, "POST", "/oauth/token", nil, form, false, &result)
	m.state.NextPollAt = time.Now().Add(time.Duration(m.state.PollInterval) * time.Second)
	if err != nil {
		return m.authorizationErrorLocked(ctx, err)
	}
	if len(result.AccessToken) != 64 || result.TokenType != "Bearer" || result.Scope != "publications:write" {
		return Status{}, failure(502, "hub_response", "Hub returned an invalid authorization response.")
	}
	m.state.AccessToken = result.AccessToken
	m.clearDeviceLocked()
	if err = m.persistLocked(); err != nil {
		return Status{}, err
	}
	return m.statusLocked(ctx, true)
}
func (m *Manager) authorizationErrorLocked(ctx context.Context, err error) (Status, error) {
	var response *apiError
	if errors.As(err, &response) {
		switch response.Code {
		case "slow_down":
			m.state.PollInterval += 5
			m.state.NextPollAt = time.Now().Add(time.Duration(m.state.PollInterval) * time.Second)
		case "authorization_pending":
		case "access_denied", "expired_token", "invalid_grant":
			m.clearDeviceLocked()
			_ = m.persistLocked()
			return Status{}, failure(409, "hub_authorization", "The connection was denied or expired. Start sign-in again.")
		default:
			return Status{}, err
		}
		if err = m.persistLocked(); err != nil {
			return Status{}, err
		}
		return m.statusLocked(ctx, false)
	}
	return Status{}, err
}
