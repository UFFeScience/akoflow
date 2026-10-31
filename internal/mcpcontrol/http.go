package mcpcontrol

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"mime"
	"net/http"
	"strings"
	"time"
)

func (m *Manager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	parts := strings.Split(token, "_")
	m.mu.Lock()
	if !m.enabled || m.closed {
		m.mu.Unlock()
		http.Error(w, "Desktop MCP is disabled", http.StatusServiceUnavailable)
		return
	}
	var entry *client
	if len(parts) == 3 && parts[0] == "akmcp" {
		entry = m.clients[parts[1]]
	}
	if entry == nil {
		legacy := m.options.Legacy
		m.mu.Unlock()
		if legacy != nil {
			legacy.ServeHTTP(w, r)
			return
		}
		unauthorized(w)
		return
	}
	expected := sha256.Sum256([]byte(entry.token))
	supplied := sha256.Sum256([]byte(token))
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || subtle.ConstantTimeCompare(expected[:], supplied[:]) != 1 {
		m.mu.Unlock()
		unauthorized(w)
		return
	}
	entry.active.Add(1)
	clientContext := entry.ctx
	handler := entry.handler
	m.mu.Unlock()
	defer entry.active.Done()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stop := context.AfterFunc(clientContext, cancel)
	defer stop()
	recorder := &statusWriter{ResponseWriter: w, status: 200}
	handler.ServeHTTP(recorder, r.WithContext(ctx))
	if recorder.status >= 200 && recorder.status < 300 && r.Method == "POST" {
		m.mu.Lock()
		if m.clients[entry.info.ID] == entry {
			now := time.Now().UTC()
			entry.info.LastConnectedAt = &now
		}
		m.mu.Unlock()
	}
}
func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	http.Error(w, "MCP connection credential is invalid or revoked", http.StatusUnauthorized)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Flush()                      { _ = http.NewResponseController(w.ResponseWriter).Flush() }
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (m *Manager) ControlHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /akoflow-api/mcp/status/", m.status)
	mux.HandleFunc("PUT /akoflow-api/mcp/settings/", m.settings)
	mux.HandleFunc("POST /akoflow-api/mcp/clients/", m.create)
	mux.HandleFunc("DELETE /akoflow-api/mcp/clients/{clientId}/", m.revoke)
	mux.HandleFunc("POST /akoflow-api/mcp/clients/{clientId}/rotate/", m.rotate)
	mux.HandleFunc("POST /akoflow-api/mcp/clients/{clientId}/probe/", m.probe)
	mux.HandleFunc("GET /akoflow-api/mcp/credentials/{clientId}/", m.credential)
	return mux
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media != "application/json" {
		fail(w, 415, "Content-Type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil {
		fail(w, 400, "Invalid MCP settings")
		return false
	}
	return true
}
func (m *Manager) status(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	state := m.stateLocked()
	m.mu.Unlock()
	writeJSON(w, 200, map[string]any{"enabled": state.Enabled, "clients": state.Clients, "endpoint": "/mcp", "available": true, "maxClients": 32})
}
