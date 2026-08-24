package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type CatalogResolver struct {
	runtimes   *Registry
	catalog    ports.EnvironmentCatalog
	factories  map[domain.RuntimeDriver]ConnectionAdapterFactory
	configured sync.Map
}

// ConnectionAdapterFactory builds an adapter from the immutable runtime and
// connection stored in an environment version. Credentials remain referenced
// by the connection; they are never copied into a schedule or an execution.
type ConnectionAdapterFactory interface {
	Driver() domain.RuntimeDriver
	Build(domain.EnvironmentRuntime, domain.EnvironmentConnection) (ports.RuntimeAdapter, error)
}

func NewCatalogResolver(runtimes *Registry, catalog ports.EnvironmentCatalog, factories ...ConnectionAdapterFactory) *CatalogResolver {
	resolver := &CatalogResolver{runtimes: runtimes, catalog: catalog,
		factories: make(map[domain.RuntimeDriver]ConnectionAdapterFactory)}
	for _, factory := range factories {
		if factory != nil {
			resolver.factories[factory.Driver()] = factory
		}
	}
	return resolver
}

func (r *CatalogResolver) Resolve(mode domain.ExecutionMode, runtimeID string) (ports.RuntimeAdapter, error) {
	adapter, err := r.runtimes.Resolve(mode, runtimeID)
	if err == nil || r.catalog == nil {
		return adapter, err
	}
	definitions, catalogErr := r.catalog.List(context.Background())
	if catalogErr != nil {
		return nil, fmt.Errorf("resolve runtime %q from environment catalog: %w", runtimeID, catalogErr)
	}
	for _, definition := range definitions {
		for _, runtime := range definition.Runtimes {
			if runtime.ID == runtimeID {
				if factory := r.factories[runtime.Driver]; factory != nil {
					connectionID, _ := runtime.Configuration["connectionId"].(string)
					if connectionID == "" {
						return nil, fmt.Errorf("runtime %q requires configuration.connectionId", runtimeID)
					}
					for _, connection := range definition.Connections {
						if connection.ID != connectionID {
							continue
						}
						cacheKey := string(mode) + "\\x00" + runtimeID
						if cached, ok := r.configured.Load(cacheKey); ok {
							return cached.(ports.RuntimeAdapter), nil
						}
						configured, buildErr := factory.Build(runtime, connection)
						if buildErr != nil {
							return nil, fmt.Errorf("configure runtime %q: %w", runtimeID, buildErr)
						}
						actual, _ := r.configured.LoadOrStore(cacheKey, configured)
						return actual.(ports.RuntimeAdapter), nil
					}
					return nil, fmt.Errorf("runtime %q references unknown connection %q", runtimeID, connectionID)
				}
				return r.runtimes.Resolve(mode, string(runtime.Driver))
			}
		}
	}
	return nil, err
}

type Registry struct {
	mu       sync.RWMutex
	runtimes map[string]ports.RuntimeAdapter
}

func New() *Registry {
	return &Registry{runtimes: make(map[string]ports.RuntimeAdapter)}
}

func (r *Registry) Register(runtimeID string, adapter ports.RuntimeAdapter) error {
	if runtimeID == "" || adapter == nil {
		return fmt.Errorf("runtime id and adapter are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	modes := adapter.Modes()
	if len(modes) == 0 {
		return fmt.Errorf("runtime %q does not declare execution modes", runtimeID)
	}
	for _, mode := range modes {
		key := runtimeKey(mode, runtimeID)
		if _, exists := r.runtimes[key]; exists {
			return fmt.Errorf("runtime %q is already registered for mode %q", runtimeID, mode)
		}
	}
	for _, mode := range modes {
		r.runtimes[runtimeKey(mode, runtimeID)] = adapter
	}
	return nil
}

func (r *Registry) Resolve(mode domain.ExecutionMode, runtimeID string) (ports.RuntimeAdapter, error) {
	r.mu.RLock()
	adapter := r.runtimes[runtimeKey(mode, runtimeID)]
	if adapter == nil {
		adapter = r.runtimes[runtimeKey(mode, "*")]
	}
	r.mu.RUnlock()
	if adapter == nil {
		return nil, fmt.Errorf("runtime %q is not registered for mode %q", runtimeID, mode)
	}
	return adapter, nil
}

func runtimeKey(mode domain.ExecutionMode, runtimeID string) string {
	return string(mode) + "\x00" + runtimeID
}
