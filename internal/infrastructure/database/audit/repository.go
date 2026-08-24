package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
)

type Repository struct{ db *sql.DB }

var _ ports.AuditStore = (*Repository)(nil)

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) RecordAuditEvent(ctx context.Context, event domainaudit.Event) error {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO audit_events
		(id,event_type,actor_id,actor_type,environment_id,resource_id,connection_id,runtime_id,
		session_id,execution_id,external_id,outcome,summary,metadata,occurred_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, event.ID, event.EventType, event.ActorID, event.ActorType,
		event.EnvironmentID, event.ResourceID, event.ConnectionID, event.RuntimeID, event.SessionID,
		event.ExecutionID, event.ExternalID, event.Outcome, event.Summary, string(metadata), event.OccurredAt)
	return err
}

func (r *Repository) ListAuditEvents(ctx context.Context, filter domainaudit.Filter) ([]domainaudit.Event, error) {
	query := `SELECT id,event_type,actor_id,actor_type,environment_id,resource_id,connection_id,runtime_id,
		session_id,execution_id,external_id,outcome,summary,metadata,occurred_at FROM audit_events WHERE 1=1`
	args := []any{}
	for _, item := range []struct{ column, value string }{
		{"event_type", filter.EventType}, {"environment_id", filter.EnvironmentID}, {"resource_id", filter.ResourceID},
		{"connection_id", filter.ConnectionID}, {"session_id", filter.SessionID}, {"execution_id", filter.ExecutionID},
		{"outcome", string(filter.Outcome)},
	} {
		if strings.TrimSpace(item.value) != "" {
			query += " AND " + item.column + "=?"
			args = append(args, item.value)
		}
	}
	limit := filter.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	query += " ORDER BY occurred_at DESC, id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domainaudit.Event{}
	for rows.Next() {
		var event domainaudit.Event
		var outcome, metadata string
		if err := rows.Scan(&event.ID, &event.EventType, &event.ActorID, &event.ActorType, &event.EnvironmentID,
			&event.ResourceID, &event.ConnectionID, &event.RuntimeID, &event.SessionID, &event.ExecutionID, &event.ExternalID,
			&outcome, &event.Summary, &metadata, &event.OccurredAt); err != nil {
			return nil, err
		}
		event.Outcome = domainaudit.Outcome(outcome)
		if metadata != "" {
			if err := json.Unmarshal([]byte(metadata), &event.Metadata); err != nil {
				return nil, err
			}
		}
		items = append(items, event)
	}
	return items, rows.Err()
}
