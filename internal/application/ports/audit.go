package ports

import (
	"context"

	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
)

type AuditStore interface {
	RecordAuditEvent(context.Context, domainaudit.Event) error
	ListAuditEvents(context.Context, domainaudit.Filter) ([]domainaudit.Event, error)
}

type AuditExplorer interface {
	ListAuditFacet(context.Context, domainaudit.Filter, string, string, int) (domainaudit.FacetPage, error)
	SummarizeAudit(context.Context, domainaudit.Filter) (domainaudit.Summary, error)
}
