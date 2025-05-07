package runtime_entity

import "strings"

type Runtime struct {
	name      string
	status    int
	metadata  map[string]string
	createdAt string
	updatedAt string
}

func NewRuntime(name string, status int, metadata map[string]string, createdAt string, updatedAt string) *Runtime {
	return &Runtime{
		name:      name,
		status:    status,
		metadata:  metadata,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}
func (r *Runtime) GetName() string {
	return r.name
}

func (r *Runtime) GetStatus() int {
	return r.status
}

func (r *Runtime) GetMetadata() map[string]string {
	return r.metadata
}

// GetMetadataApiServerToken returns the API server token from the metadata map.
// It converts the key to uppercase to ensure consistency in key retrieval.
// If the key is not found, it returns an empty string.
func (r *Runtime) GetMetadataApiServerToken() string {
	key := r.name + "_API_SERVER_TOKEN"
	key = strings.ToUpper(key)
	if token, ok := r.metadata[key]; ok {
		return token
	}
	return ""
}

// GetMetadataApiServerHost returns the API server URL from the metadata map.
// It converts the key to uppercase to ensure consistency in key retrieval.
// If the key is not found, it returns an empty string.
func (r *Runtime) GetMetadataApiServerHost() string {
	key := r.name + "_API_SERVER_HOST"
	key = strings.ToUpper(key)
	if url, ok := r.metadata[key]; ok {
		return url
	}
	return ""
}

func (r *Runtime) GetCreatedAt() string {
	return r.createdAt
}

func (r *Runtime) GetUpdatedAt() string {
	return r.updatedAt
}
