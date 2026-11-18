package hubtelemetry

import (
	"context"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain/audit"
)

type AuditStore struct {
	ports.AuditStore
	Sender  *Sender
	Release string
}

func (s AuditStore) RecordAuditEvent(ctx context.Context, event audit.Event) error {
	if err := s.AuditStore.RecordAuditEvent(ctx, event); err != nil {
		return err
	}
	s.Sender.RecordAccess(event, s.Release)
	if event.EventType == "mcp.request" && event.InternalFailure {
		s.Sender.recordInternalFailure(event, s.Release, "mcp.internal_error", event.Method)
	}
	return nil
}
