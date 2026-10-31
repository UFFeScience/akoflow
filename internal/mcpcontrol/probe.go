package mcpcontrol

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type tokenTransport struct{ token string }

func (t tokenTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	request := r.Clone(r.Context())
	request.Header = r.Header.Clone()
	request.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(request)
}
func (m *Manager) probe(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	entry := m.clients[r.PathValue("clientId")]
	if entry == nil {
		m.mu.Unlock()
		fail(w, 404, "MCP app connection not found")
		return
	}
	if !m.enabled {
		m.mu.Unlock()
		fail(w, 409, "Enable MCP before testing a connection")
		return
	}
	id := entry.info.ID
	token := entry.token
	m.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	base, _ := url.Parse(m.options.APIURL)
	base.Path = "/mcp"
	client := mcp.NewClient(&mcp.Implementation{Name: "Akoflow Desktop connection test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: base.String(), HTTPClient: &http.Client{Transport: tokenTransport{token}, Timeout: 15 * time.Second},
		DisableStandaloneSSE: true, MaxRetries: -1,
	}, nil)
	if err != nil {
		fail(w, 502, "MCP handshake failed. Keep the daemon running and try again.")
		return
	}
	defer session.Close()
	count := 0
	for _, err := range session.Tools(ctx, nil) {
		if err != nil {
			fail(w, 502, "MCP tool discovery failed")
			return
		}
		count++
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_instance", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		fail(w, 502, "MCP connected, but the daemon identity could not be read")
		return
	}
	now := time.Now().UTC()
	m.mu.Lock()
	if current := m.clients[id]; current == entry && current.token == token {
		entry.info.LastVerifiedAt = &now
		_ = m.persistLocked()
	}
	m.mu.Unlock()
	writeJSON(w, 200, map[string]any{
		"healthy": true, "toolCount": count, "protocolVersion": session.InitializeResult().ProtocolVersion,
		"serverName": "akoflow", "checkedAt": now,
		"checks": []string{"Credential accepted", "MCP handshake completed", "Tools discovered", "Daemon identity read"},
	})
}
