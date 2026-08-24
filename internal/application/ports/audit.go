package ports

import (
	"context"

	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
)

type AuditStore interface {
	RecordAuditEvent(context.Context, domainaudit.Event) error
	ListAuditEvents(context.Context, domainaudit.Filter) ([]domainaudit.Event, error)
}
