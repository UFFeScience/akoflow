package mcpcontrol

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/UFFeScience/akoflow/internal/mcpbridge"
)

func (m *Manager) settings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Enabled bool `json:"enabled"`
	}
	if !decode(w, r, &input) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	previous := m.enabled
	m.enabled = input.Enabled
	if err := m.persistLocked(); err != nil {
		m.enabled = previous
		fail(w, 500, "Could not save MCP settings")
		return
	}
	if previous != m.enabled {
		for _, entry := range m.clients {
			entry.cancel()
			if m.enabled {
				entry.ctx, entry.cancel = context.WithCancel(context.Background())
			}
		}
	}
	writeJSON(w, 200, map[string]bool{"enabled": m.enabled})
}
func (m *Manager) create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name       string               `json:"name"`
		App        string               `json:"app"`
		Permission mcpbridge.Permission `json:"permission"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 64 || strings.IndexFunc(input.Name, unicode.IsControl) >= 0 {
		fail(w, 422, "Use an app name between 1 and 64 characters")
		return
	}
	if !validApp(input.App) || !validPermission(input.Permission) {
		fail(w, 422, "Choose a supported app and permission")
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.enabled {
		fail(w, 409, "Enable MCP before adding an app")
		return
	}
	if len(m.clients) >= 32 {
		fail(w, 409, "The maximum number of MCP app connections has been reached")
		return
	}
	id := randomValue(16)
	token := "akmcp_" + id + "_" + randomValue(32)
	info := Connection{ID: id, Name: input.Name, App: input.App, Permission: input.Permission, CredentialRef: "mcp:" + id, CreatedAt: time.Now().UTC()}
	entry, err := m.newClient(info, token)
	if err != nil {
		fail(w, 500, "Could not prepare the app connection")
		return
	}
	if err = privateWrite(m.credentialPath(id), []byte(token)); err != nil {
		entry.bridge.Close()
		fail(w, 500, "Could not save app credential")
		return
	}
	m.clients[id] = entry
	if err = m.persistLocked(); err != nil {
		delete(m.clients, id)
		os.Remove(m.credentialPath(id))
		entry.bridge.Close()
		fail(w, 500, "Could not save app connection")
		return
	}
	writeJSON(w, 201, entry.info)
}
func validApp(app string) bool {
	return app == "codex" || app == "claude-desktop" || app == "claude-code" || app == "other"
}
func validPermission(permission mcpbridge.Permission) bool {
	return permission == mcpbridge.Read || permission == mcpbridge.Write || permission == mcpbridge.Admin
}
func (m *Manager) revoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("clientId")
	m.mu.Lock()
	entry := m.clients[id]
	if entry == nil {
		m.mu.Unlock()
		fail(w, 404, "MCP app connection not found")
		return
	}
	delete(m.clients, id)
	if err := m.persistLocked(); err != nil {
		m.clients[id] = entry
		m.mu.Unlock()
		fail(w, 500, "Could not revoke app access")
		return
	}
	entry.cancel()
	m.mu.Unlock()
	_ = os.Remove(m.credentialPath(id))
	go func() { entry.active.Wait(); entry.bridge.Close() }()
	w.WriteHeader(204)
}
func (m *Manager) credential(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := m.clients[r.PathValue("clientId")]
	if entry == nil {
		fail(w, 404, "MCP app connection not found")
		return
	}
	writeJSON(w, 200, map[string]string{"clientId": entry.info.ID, "credentialRef": entry.info.CredentialRef, "token": entry.token})
}
func (m *Manager) rotate(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := m.clients[r.PathValue("clientId")]
	if entry == nil {
		fail(w, 404, "MCP app connection not found")
		return
	}
	token := "akmcp_" + entry.info.ID + "_" + randomValue(32)
	handler, err := entry.bridge.HTTPHandler(mcpbridge.HTTPOptions{Token: token})
	if err != nil {
		fail(w, 500, "Could not rotate app credential")
		return
	}
	if err = privateWrite(m.credentialPath(entry.info.ID), []byte(token)); err != nil {
		fail(w, 500, "Could not save app credential")
		return
	}
	entry.cancel()
	entry.ctx, entry.cancel = context.WithCancel(context.Background())
	entry.token = token
	entry.handler = handler
	entry.info.LastVerifiedAt = nil
	if err = m.persistLocked(); err != nil {
		fail(w, 500, "Credential rotated, but its status could not be saved")
		return
	}
	writeJSON(w, 200, entry.info)
}
