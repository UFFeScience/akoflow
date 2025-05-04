package runtime_entity

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

func (r *Runtime) GetCreatedAt() string {
	return r.createdAt
}

func (r *Runtime) GetUpdatedAt() string {
	return r.updatedAt
}
