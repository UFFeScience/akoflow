package hubpublish

import "net/url"

func (m *Manager) configureService(saved bool) error {
	origin := ""
	var err error
	if m.options.HubURL != "" {
		origin, err = validateOrigin(m.options.HubURL)
		if err != nil {
			return err
		}
	}
	m.options.HubURL = origin
	if m.options.BrowserURL == "" {
		m.options.BrowserURL = origin
	}
	if m.options.BrowserURL != "" {
		m.options.BrowserURL, err = validateOrigin(m.options.BrowserURL)
		if err != nil {
			return err
		}
	}
	changed := m.state.HubURL != origin
	if changed {
		m.clearAuthorizationLocked()
		m.state.Enabled = false
		m.state.DefaultPublicationID = ""
	}
	m.state.HubURL = origin
	if changed && saved && !m.options.ReadOnly {
		return m.persistLocked()
	}
	return nil
}

func (m *Manager) browserAddress(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || !sameOrigin(value, m.options.HubURL) {
		return ""
	}
	return m.options.BrowserURL + parsed.RequestURI()
}
