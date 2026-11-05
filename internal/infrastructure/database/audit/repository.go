package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
)

type Repository struct{ db *sql.DB }

var _ ports.AuditStore = (*Repository)(nil)

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) RecordAuditEvent(ctx context.Context, event domainaudit.Event) error {
	err := Append(ctx, r.db, event)
	if err != nil {
		log.Print("audit event persistence failed")
	}
	return err
}

type Execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// Append participates in the owning aggregate transaction. A failed audit write
// rolls back the lifecycle transition rather than silently dropping evidence.
func Append(ctx context.Context, executor Execer, event domainaudit.Event) error {
	event = sanitize(event)
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}
	_, err = executor.ExecContext(ctx, insertAuditSQL,
		event.ID,
		event.EventType,
		event.ActorID,
		event.ActorType,
		event.EnvironmentID, event.ResourceID, event.ConnectionID, event.RuntimeID, event.SessionID,
		event.ExecutionID,
		event.ExternalID,
		event.Outcome,
		event.Summary,
		string(metadata),
		event.OccurredAt,
		event.RequestID,
		event.CorrelationID,
		event.Transport,
		event.Source,
		event.Method,
		event.Route,
		event.Status,
		event.DurationMS,
		event.AuthOutcome,
		event.ActivityID,
		event.Attempt,
		event.OperationID)
	return err
}

func (r *Repository) ListAuditEvents(ctx context.Context, filter domainaudit.Filter) ([]domainaudit.Event, error) {
	query, args, err := auditQuery(filter)
	if err != nil {
		return nil, err
	}
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
			&outcome,
			&event.Summary,
			&metadata,
			&event.OccurredAt,
			&event.RequestID,
			&event.CorrelationID,
			&event.Transport,
			&event.Source,
			&event.Method,
			&event.Route,
			&event.Status,
			&event.DurationMS,
			&event.AuthOutcome,
			&event.ActivityID,
			&event.Attempt,
			&event.OperationID); err != nil {
			return nil, err
		}
		event.Outcome = domainaudit.Outcome(outcome)
		if metadata != "" {
			if err := json.Unmarshal([]byte(metadata), &event.Metadata); err != nil {
				return nil, err
			}
		}
		items = append(items, sanitize(event))
	}
	return items, rows.Err()
}

func auditQuery(filter domainaudit.Filter) (string, []any, error) {
	query := selectAuditSQL
	prefix, err := familyPrefix(filter.Category)
	if err != nil {
		return "", nil, err
	}
	args := []any{}
	if prefix != "" {
		query += " AND event_type>=? AND event_type<?"
		args = append(args, prefix+".", prefix+"/")
	}
	for _, item := range []struct{ column, value string }{
		{"event_type", filter.EventType}, {"environment_id", filter.EnvironmentID}, {"resource_id", filter.ResourceID},
		{"connection_id", filter.ConnectionID}, {"session_id", filter.SessionID}, {"execution_id", filter.ExecutionID},
		{"outcome", string(filter.Outcome)},
		{"request_id", filter.RequestID},
		{"correlation_id", filter.CorrelationID},
		{"actor_id", filter.ActorID},
		{"method", filter.Method},
		{"route", filter.Route},
		{"transport", filter.Transport},
		{"runtime_id", filter.RuntimeID},
		{"activity_id", filter.ActivityID},
		{"operation_id", filter.OperationID},
	} {
		if strings.TrimSpace(item.value) != "" {
			query += " AND " + item.column + "=?"
			args = append(args, item.value)
		}
	}
	if filter.Status != 0 {
		query += " AND status=?"
		args = append(args, filter.Status)
	}
	for _, boundary := range []struct{ value, clause string }{{filter.From, " AND occurred_at>=?"}, {filter.To, " AND occurred_at<=?"}} {
		if boundary.value != "" {
			query += boundary.clause
			value, err := time.Parse(time.RFC3339Nano, boundary.value)
			if err != nil {
				return "", nil, err
			}
			args = append(args, value.UTC())
		}
	}
	if filter.Before != "" {
		query += " AND (occurred_at,id) < (SELECT occurred_at,id FROM audit_events WHERE id=?)"
		args = append(args, filter.Before)
	}
	limit := filter.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	query += " ORDER BY occurred_at DESC, id DESC LIMIT ?"
	args = append(args, limit)
	return query, args, nil
}

const selectAuditSQL = `SELECT id,event_type,actor_id,actor_type,environment_id,resource_id,connection_id,runtime_id,
		session_id,execution_id,external_id,outcome,summary,metadata,occurred_at,request_id,correlation_id,transport,source,method,route,status,duration_ms,auth_outcome,activity_id,attempt,operation_id FROM audit_events WHERE 1=1`

// sanitize is an allowlist boundary for durable audit records.
func sanitize(event domainaudit.Event) domainaudit.Event {
	event.Summary = event.EventType
	allowed := map[string]bool{"nodeCount": true, "logReference": true, "entityType": true, "entityId": true, "commandType": true, "commandId": true, "category": true}
	metadata := map[string]any{}
	for k, v := range event.Metadata {
		if allowed[k] {
			switch value := v.(type) {
			case string:
				metadata[k] = clean(value)
			case int, int64, float64, bool:
				metadata[k] = value
			}
		}
	}
	event.Metadata = metadata
	event.RequestID = clean(event.RequestID)
	event.CorrelationID = clean(event.CorrelationID)
	event.Transport = clean(event.Transport)
	event.Source = clean(event.Source)
	event.Method = clean(event.Method)
	event.Route = clean(event.Route)
	event.AuthOutcome = clean(event.AuthOutcome)
	event.ActivityID = clean(event.ActivityID)
	event.OperationID = clean(event.OperationID)
	event.ID = clean(event.ID)
	event.EventType = clean(event.EventType)
	event.ActorID = clean(event.ActorID)
	event.ActorType = clean(event.ActorType)
	event.EnvironmentID = clean(event.EnvironmentID)
	event.ResourceID = clean(event.ResourceID)
	event.ConnectionID = clean(event.ConnectionID)
	event.RuntimeID = clean(event.RuntimeID)
	event.SessionID = clean(event.SessionID)
	event.ExecutionID = clean(event.ExecutionID)
	event.ExternalID = clean(event.ExternalID)
	event.Summary = clean(event.Summary)
	return event
}
func clean(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == 0x2028 || r == 0x2029 {
			return -1
		}
		return r
	}, value)
	if len(value) > 256 {
		value = strings.ToValidUTF8(value[:256], "")
	}
	return value
}

const insertAuditSQL = `INSERT INTO audit_events
		(id,event_type,actor_id,actor_type,environment_id,resource_id,connection_id,runtime_id,
		session_id,execution_id,external_id,outcome,summary,metadata,occurred_at,request_id,correlation_id,transport,source,method,route,status,duration_ms,auth_outcome,activity_id,attempt,operation_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET event_type=excluded.event_type,
actor_id=excluded.actor_id,
actor_type=excluded.actor_type,
environment_id=excluded.environment_id,
resource_id=excluded.resource_id,
connection_id=excluded.connection_id,
runtime_id=excluded.runtime_id,
session_id=excluded.session_id,
execution_id=excluded.execution_id,
external_id=excluded.external_id,
outcome=excluded.outcome,
summary=excluded.summary,
metadata=excluded.metadata,
request_id=excluded.request_id,
correlation_id=excluded.correlation_id,
transport=excluded.transport,
source=excluded.source,
method=excluded.method,
route=excluded.route,
status=excluded.status,
duration_ms=excluded.duration_ms,
auth_outcome=excluded.auth_outcome,
activity_id=excluded.activity_id,
attempt=excluded.attempt,
operation_id=excluded.operation_id
WHERE audit_events.event_type IN ('api.access','mcp.request')`

func familyPrefix(category string) (string, error) {
	if len(category) > 64 {
		return "", fmt.Errorf("invalid audit event family")
	}
	for _, letter := range category {
		if (letter < 'a' || letter > 'z') && (letter < '0' || letter > '9') && letter != '_' && letter != '-' {
			return "", fmt.Errorf("invalid audit event family")
		}
	}
	return category, nil
}
