package planning

import (
	"context"
	"time"

	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	dbaudit "github.com/UFFeScience/akoflow/internal/infrastructure/database/audit"
)

func auditPlanning(ctx context.Context, tx dbaudit.Execer, kind, id, sessionID, state string) error {
	outcome := domainaudit.OutcomeStarted
	switch state {
	case "completed", "selected", "candidate":
		outcome = domainaudit.OutcomeSucceeded
	case "failed", "cancelled":
		outcome = domainaudit.OutcomeFailed
	}
	return dbaudit.AppendLifecycle(ctx, tx, domainaudit.Event{
		ID: "planning:" + kind + ":" + id + ":" + state, EventType: "planning." + kind + "." + state,
		ActorID: "daemon", ActorType: "service_client", AuthOutcome: "internal",
		Source: "planning", Transport: "internal", SessionID: sessionID, OperationID: id,
		CorrelationID: "planning:" + sessionID, Outcome: outcome, OccurredAt: time.Now().UTC(),
		Metadata: map[string]any{"entityType": kind, "entityId": id},
	}, "planning_session", sessionID)
}

func (r *Repository) planningTransition(ctx context.Context, kind, state, query string, args ...any) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	parent := "id"
	if kind == "algorithm" {
		parent = "planning_session_id"
	}
	rows, err := tx.QueryContext(ctx, query+" RETURNING id,"+parent, args...)
	if err != nil {
		return err
	}
	type target struct{ id, parent string }
	targets := []target{}
	for rows.Next() {
		var item target
		if err := rows.Scan(&item.id, &item.parent); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range targets {
		if err := auditPlanning(ctx, tx, kind, item.id, item.parent, state); err != nil {
			return err
		}
	}
	return tx.Commit()
}
