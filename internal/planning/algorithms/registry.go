package algorithms

import (
	"fmt"
	"sort"
	"sync"

	"github.com/UFFeScience/akoflow/internal/application/ports"
)

type Registry struct {
	mu         sync.RWMutex
	schedulers map[string]ports.Scheduler
}

func NewRegistry(schedulers ...ports.Scheduler) (*Registry, error) {
	registry := &Registry{schedulers: map[string]ports.Scheduler{}}
	for _, scheduler := range schedulers {
		if err := registry.Register(scheduler); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (r *Registry) Register(scheduler ports.Scheduler) error {
	if scheduler == nil || scheduler.Descriptor().ID == "" {
		return fmt.Errorf("scheduler and id are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	id := scheduler.Descriptor().ID
	if _, exists := r.schedulers[id]; exists {
		return fmt.Errorf("scheduler %q already registered", id)
	}
	r.schedulers[id] = scheduler
	return nil
}

func (r *Registry) Find(id string) (ports.Scheduler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	scheduler, ok := r.schedulers[id]
	return scheduler, ok
}

func (r *Registry) List() []ports.SchedulerDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]ports.SchedulerDescriptor, 0, len(r.schedulers))
	for _, scheduler := range r.schedulers {
		items = append(items, scheduler.Descriptor())
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}
