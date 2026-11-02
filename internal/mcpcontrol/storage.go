package mcpcontrol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

func (m *Manager) credentialPath(id string) string {
	return filepath.Join(m.options.Directory, "credentials", id)
}
func (m *Manager) stateLocked() savedState {
	state := savedState{Enabled: m.enabled, Clients: []Connection{}}
	for _, entry := range m.clients {
		state.Clients = append(state.Clients, entry.info)
	}
	sort.Slice(state.Clients, func(i, j int) bool { return state.Clients[i].CreatedAt.Before(state.Clients[j].CreatedAt) })
	return state
}
func (m *Manager) persistLocked() error {
	data, err := json.MarshalIndent(m.stateLocked(), "", "  ")
	if err != nil {
		return err
	}
	return privateWrite(filepath.Join(m.options.Directory, "desktop-settings.json"), data)
}
func privateWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".mcp-")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}
