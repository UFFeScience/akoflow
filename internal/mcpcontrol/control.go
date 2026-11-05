// Package mcpcontrol manages desktop MCP connections without exposing lifecycle
// or credential-management endpoints as tools to MCP clients.
package mcpcontrol

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/UFFeScience/akoflow/internal/mcpbridge"
)

type Options struct {
	Audit            ports.AuditStore
	APIURL           string
	APIToken         string
	Directory        string
	InitiallyEnabled bool
	Legacy           http.Handler
}
type Connection struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	App             string               `json:"app"`
	Permission      mcpbridge.Permission `json:"permission"`
	CredentialRef   string               `json:"credentialRef"`
	CreatedAt       time.Time            `json:"createdAt"`
	LastConnectedAt *time.Time           `json:"lastConnectedAt,omitempty"`
	LastVerifiedAt  *time.Time           `json:"lastVerifiedAt,omitempty"`
	ToolCount       int                  `json:"toolCount"`
}
type savedState struct {
	Enabled bool         `json:"enabled"`
	Clients []Connection `json:"clients"`
}
type client struct {
	info    Connection
	token   string
	bridge  *mcpbridge.Bridge
	handler http.Handler
	ctx     context.Context
	cancel  context.CancelFunc
	active  sync.WaitGroup
}
type Manager struct {
	options Options
	mu      sync.Mutex
	enabled bool
	clients map[string]*client
	closed  bool
}

func New(options Options) (*Manager, error) {
	if options.Directory == "" {
		return nil, errors.New("MCP management requires a private state directory")
	}
	if err := os.MkdirAll(options.Directory, 0700); err != nil {
		return nil, err
	}
	manager := &Manager{options: options, enabled: options.InitiallyEnabled, clients: map[string]*client{}}
	if err := manager.applyPendingReset(); err != nil {
		return nil, err
	}
	if err := manager.load(); err != nil {
		manager.Close()
		return nil, err
	}
	return manager, nil
}
func (m *Manager) load() error {
	data, err := os.ReadFile(filepath.Join(m.options.Directory, "desktop-settings.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var state savedState
	if err = json.Unmarshal(data, &state); err != nil {
		return err
	}
	m.enabled = state.Enabled
	for _, info := range state.Clients {
		if !validID(info.ID) {
			return errors.New("invalid saved MCP connection")
		}
		token, err := os.ReadFile(m.credentialPath(info.ID))
		if err != nil {
			return err
		}
		entry, err := m.newClient(info, strings.TrimSpace(string(token)))
		if err != nil {
			return err
		}
		m.clients[info.ID] = entry
	}
	return nil
}
func (m *Manager) newClient(info Connection, token string) (*client, error) {
	bridge, err := mcpbridge.New(mcpbridge.Options{
		Audit: m.options.Audit, ClientID: info.ID, APIURL: m.options.APIURL, APIToken: m.options.APIToken, Permissions: info.Permission,
		StateDirectory: filepath.Join(m.options.Directory, "sessions", info.ID),
	})
	if err != nil {
		return nil, err
	}
	handler, err := bridge.HTTPHandler(mcpbridge.HTTPOptions{Token: token})
	if err != nil {
		bridge.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	info.ToolCount = toolCount(info.Permission)
	return &client{info: info, token: token, bridge: bridge, handler: handler, ctx: ctx, cancel: cancel}, nil
}
func toolCount(permission mcpbridge.Permission) int {
	count := 2
	if permission.Allows(mcpbridge.Write) {
		count += 3
	}
	if permission == mcpbridge.Admin {
		count++
	}
	for _, operation := range mcpbridge.Catalog() {
		if permission.Allows(operation.Permission()) {
			count++
		}
	}
	return count
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	entries := []*client{}
	for _, entry := range m.clients {
		entry.cancel()
		entries = append(entries, entry)
	}
	m.clients = map[string]*client{}
	m.mu.Unlock()
	for _, entry := range entries {
		entry.active.Wait()
		entry.bridge.Close()
	}
}
func randomValue(size int) string {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value)
}
func validID(value string) bool {
	if len(value) != 32 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
