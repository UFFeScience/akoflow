package token

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// Manager persists Kubernetes bearer tokens outside the control-plane database.
// Callers receive only a file: reference; token contents are never returned.
type Manager struct{ directory string }

func New(directory string) *Manager { return &Manager{directory: directory} }

func (m *Manager) Save(id, value string) (string, error) {
	id = strings.TrimSpace(id)
	value = strings.TrimSpace(value)
	if !validID.MatchString(id) {
		return "", fmt.Errorf("credential id must contain lowercase letters, numbers, or hyphens")
	}
	if value == "" {
		return "", fmt.Errorf("Kubernetes token is required")
	}
	if err := os.MkdirAll(m.directory, 0o700); err != nil {
		return "", fmt.Errorf("create credential directory: %w", err)
	}
	path := filepath.Join(m.directory, id+".token")
	if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("save Kubernetes token: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("secure Kubernetes token: %w", err)
	}
	return "file:" + path, nil
}
