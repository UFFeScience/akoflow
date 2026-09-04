package cloud

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

type Manager struct{ directory string }

func New(directory string) *Manager { return &Manager{directory: directory} }

func (m *Manager) Save(id, provider string, payload json.RawMessage) (string, error) {
	id, provider = strings.TrimSpace(id), strings.TrimSpace(provider)
	if !validID.MatchString(id) {
		return "", fmt.Errorf("credential id must contain lowercase letters, numbers, or hyphens")
	}
	if provider != "gcp" && provider != "aws" && provider != "azure" {
		return "", fmt.Errorf("unsupported cloud provider %q", provider)
	}
	if !json.Valid(payload) {
		return "", fmt.Errorf("cloud credential must be valid JSON")
	}
	if err := os.MkdirAll(m.directory, 0o700); err != nil {
		return "", fmt.Errorf("create cloud credential directory: %w", err)
	}
	path := filepath.Join(m.directory, id+".json")
	wrapper, err := json.Marshal(map[string]any{"provider": provider, "credential": json.RawMessage(payload)})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, wrapper, 0o600); err != nil {
		return "", fmt.Errorf("save cloud credential: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("secure cloud credential: %w", err)
	}
	return "cloud-file:" + path, nil
}

func (m *Manager) Resolve(reference string) ([]byte, error) {
	const prefix = "cloud-file:"
	if !strings.HasPrefix(reference, prefix) {
		return nil, fmt.Errorf("unsupported cloud credential reference")
	}
	path := filepath.Clean(strings.TrimPrefix(reference, prefix))
	root, err := filepath.Abs(m.directory)
	if err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if absolute != root && !strings.HasPrefix(absolute, root+string(filepath.Separator)) {
		return nil, fmt.Errorf("cloud credential is outside the managed directory")
	}
	data, err := os.ReadFile(absolute)
	if err != nil {
		return nil, fmt.Errorf("read cloud credential: %w", err)
	}
	var wrapper struct {
		Credential json.RawMessage `json:"credential"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil || len(wrapper.Credential) == 0 {
		return nil, fmt.Errorf("invalid stored cloud credential")
	}
	return wrapper.Credential, nil
}
