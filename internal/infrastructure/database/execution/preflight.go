package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain/preflight"
)

func (r *Repository) SavePreflight(ctx context.Context, report preflight.Report) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO domain_events(id,event_type,aggregate_type,aggregate_id,payload,occurred_at)
 VALUES(?,'execution.preflight.completed','execution_preflight',?,?,?)`, report.ID, report.ID, payload, report.CreatedAt); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM domain_events WHERE event_type='execution.preflight.completed'
 AND id NOT IN (SELECT id FROM domain_events WHERE event_type='execution.preflight.completed'
 ORDER BY occurred_at DESC LIMIT 1000) AND occurred_at < ?`, time.Now().UTC().Add(-24*time.Hour)); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *Repository) FindPreflight(ctx context.Context, id string) (*preflight.Report, error) {
	var payload []byte
	err := r.db.QueryRowContext(ctx, `SELECT payload FROM domain_events WHERE id=? AND event_type='execution.preflight.completed'`, id).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var report preflight.Report
	if err := json.Unmarshal(payload, &report); err != nil {
		return nil, err
	}
	return &report, nil
}
