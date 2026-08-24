package events

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	domainevents "github.com/UFFeScience/akoflow/internal/domain/events"
)

type Execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func Append(ctx context.Context, executor Execer, event domainevents.Event) error {
	if executor == nil || event.Type == "" || event.AggregateType == "" || event.AggregateID == "" {
		return fmt.Errorf("domain event type and aggregate are required")
	}
	if event.ID == "" {
		event.ID = newID()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal domain event: %w", err)
	}
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal domain event metadata: %w", err)
	}
	if _, err := executor.ExecContext(ctx, `INSERT INTO domain_events (
		id, event_type, aggregate_type, aggregate_id, payload, occurred_at, metadata
	) VALUES (?, ?, ?, ?, ?, ?, ?)`, event.ID, event.Type, event.AggregateType,
		event.AggregateID, payload, event.OccurredAt, metadata); err != nil {
		return fmt.Errorf("append domain event: %w", err)
	}
	if _, err := executor.ExecContext(ctx, `INSERT INTO queue_jobs (
		id, category, event_type, aggregate_type, aggregate_id, payload, status,
		priority, available_at, attempts, max_attempts, idempotency_key, created_at
	) VALUES (?, 'monitoring', ?, ?, ?, ?, 'pending', 0, ?, 0, 5, ?, ?)`,
		newID(), event.Type, event.AggregateType, event.AggregateID, payload,
		event.OccurredAt, "domain-event:"+event.ID, event.OccurredAt); err != nil {
		return fmt.Errorf("enqueue domain event: %w", err)
	}
	return nil
}

func newID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
}
