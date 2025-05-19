package runtime_entity

import "strings"

type Runtime struct {
	Name      string
	Status    int
	Metadata  map[string]string
	CreatedAt string
	UpdatedAt string
}

func NewRuntime(name string, status int, metadata map[string]string, createdAt string, updatedAt string) *Runtime {
	return &Runtime{
		Name:      name,
		Status:    status,
		Metadata:  metadata,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}
func (r *Runtime) GetName() string {
	return r.Name
}

func (r *Runtime) GetStatus() int {
	return r.Status
}

func (r *Runtime) GetMetadata() map[string]string {
	return r.Metadata
}

func (r *Runtime) GetCurrentRuntimeMetadata(key string) string {
	key = r.Name + "_" + key
	key = strings.ToUpper(key)
	if metadata, ok := r.Metadata[key]; ok {
		return metadata
	}
	return ""
}

// GetMetadataApiServerToken returns the API server token from the metadata map.
// It converts the key to uppercase to ensure consistency in key retrieval.
// If the key is not found, it returns an empty string.
func (r *Runtime) GetMetadataApiServerToken() string {
	key := r.Name + "_API_SERVER_TOKEN"
	key = strings.ToUpper(key)
	if token, ok := r.Metadata[key]; ok {
		return token
	}
	return ""
}

// GetMetadataApiServerHost returns the API server URL from the metadata map.
// It converts the key to uppercase to ensure consistency in key retrieval.
// If the key is not found, it returns an empty string.
func (r *Runtime) GetMetadataApiServerHost() string {
	key := r.Name + "_API_SERVER_HOST"
	key = strings.ToUpper(key)
	if url, ok := r.Metadata[key]; ok {
		return url
	}
	return ""
}

func (r *Runtime) GetCreatedAt() string {
	return r.CreatedAt
}

func (r *Runtime) GetUpdatedAt() string {
	return r.UpdatedAt
}
