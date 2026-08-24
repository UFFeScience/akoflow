package eventloop

import (
	"context"
	"fmt"
	"sync"

	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

type Handler interface {
	Handle(context.Context, domainqueue.Job) error
}

type HandlerFunc func(context.Context, domainqueue.Job) error

func (f HandlerFunc) Handle(ctx context.Context, job domainqueue.Job) error {
	return f(ctx, job)
}

type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{handlers: make(map[string]Handler)}
}

func (d *Dispatcher) Register(eventType string, handler Handler) error {
	if eventType == "" || handler == nil {
		return fmt.Errorf("event type and handler are required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.handlers[eventType]; exists {
		return fmt.Errorf("handler already registered for %s", eventType)
	}
	d.handlers[eventType] = handler
	return nil
}

func (d *Dispatcher) Dispatch(ctx context.Context, job domainqueue.Job) error {
	d.mu.RLock()
	handler := d.handlers[job.Type]
	d.mu.RUnlock()
	if handler == nil {
		return fmt.Errorf("no handler registered for event %s", job.Type)
	}
	return handler.Handle(ctx, job)
}
