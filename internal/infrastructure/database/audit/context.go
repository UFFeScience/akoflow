package audit

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/UFFeScience/akoflow/internal/application/auditcontext"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
)

// Bind persists only server-established provenance, never a request payload.
// The owning aggregate transaction makes it available to asynchronous workers.
func Bind(ctx context.Context, executor Execer, kind, id string) error {
	trace := auditcontext.From(ctx)
	if trace == nil {
		return nil
	}
	value := *trace
	value.ActorID = clean(value.ActorID)
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = executor.ExecContext(ctx, `INSERT INTO audit_entity_context(entity_type,entity_id,provenance)
 VALUES (?,?,?) ON CONFLICT(entity_type,entity_id) DO NOTHING`, kind, id, string(payload))
	return err
}

type contextReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func AppendLifecycle(ctx context.Context, executor Execer, event domainaudit.Event, kind, id string) error {
	trace := auditcontext.From(ctx)
	if trace == nil {
		if reader, ok := executor.(contextReader); ok {
			var payload string
			err := reader.QueryRowContext(ctx, "SELECT provenance FROM audit_entity_context WHERE entity_type=? AND entity_id=?", kind, id).Scan(&payload)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			if err == nil {
				var stored auditcontext.Trace
				if err := json.Unmarshal([]byte(payload), &stored); err != nil {
					return err
				}
				trace = &stored
			}
		}
	}
	if trace != nil {
		event.RequestID, event.CorrelationID = trace.RequestID, trace.CorrelationID
		event.ActorID, event.ActorType, event.AuthOutcome = trace.ActorID, trace.ActorType, trace.AuthOutcome
	}
	return Append(ctx, executor, event)
}
