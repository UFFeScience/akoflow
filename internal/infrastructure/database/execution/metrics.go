package execution

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// Metric samples intentionally live outside the canonical bootstrap schema.
// This additive extension can be installed on an existing instance without
// recreating its database or interrupting historical runs.
func (r *Repository) EnsureMetricSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS activity_metric_samples (
		execution_run_id TEXT NOT NULL,
		activity_id TEXT NOT NULL,
		attempt INTEGER NOT NULL,
		observed_at REAL NOT NULL,
		cpu_seconds REAL NOT NULL,
		memory_bytes INTEGER NOT NULL,
		read_bytes INTEGER NOT NULL,
		write_bytes INTEGER NOT NULL,
		source TEXT NOT NULL,
		scope TEXT NOT NULL,
		PRIMARY KEY (execution_run_id, activity_id, attempt, observed_at)
	);
	CREATE INDEX IF NOT EXISTS activity_metric_samples_run_idx
		ON activity_metric_samples(execution_run_id, activity_id, attempt);`)
	if err != nil {
		return fmt.Errorf("install activity metric storage: %w", err)
	}
	return nil
}

func (r *Repository) SaveActivityMetrics(ctx context.Context, samples []domain.ActivityMetricSample) error {
	if len(samples) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statement, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO activity_metric_samples
		(execution_run_id, activity_id, attempt, observed_at, cpu_seconds,
		 memory_bytes, read_bytes, write_bytes, source, scope)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, sample := range samples {
		if sample.RunID == "" || sample.ActivityID == "" || sample.Attempt < 1 || sample.ObservedAt <= 0 {
			continue
		}
		if _, err := statement.ExecContext(ctx, sample.RunID, sample.ActivityID, sample.Attempt,
			sample.ObservedAt, sample.CPUSeconds, sample.MemoryBytes, sample.ReadBytes,
			sample.WriteBytes, sample.Source, sample.Scope); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) ListActivityMetricSummaries(ctx context.Context, runID string) ([]domain.ActivityMetricSummary, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT execution_run_id, activity_id, attempt,
		COUNT(*), MIN(observed_at), MAX(observed_at),
		MAX(cpu_seconds)-MIN(cpu_seconds), MAX(memory_bytes),
		MAX(read_bytes)-MIN(read_bytes), MAX(write_bytes)-MIN(write_bytes),
		MIN(source), MIN(scope)
		FROM activity_metric_samples WHERE execution_run_id=?
		GROUP BY execution_run_id, activity_id, attempt ORDER BY activity_id, attempt`, runID)
	if err != nil {
		if missingMetricTable(err) {
			return []domain.ActivityMetricSummary{}, nil
		}
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.ActivityMetricSummary, 0)
	for rows.Next() {
		var item domain.ActivityMetricSummary
		if err := rows.Scan(&item.RunID, &item.ActivityID, &item.Attempt,
			&item.Samples, &item.FirstObservedAt, &item.LastObservedAt,
			&item.CPUSeconds, &item.PeakMemoryBytes, &item.ReadBytes,
			&item.WriteBytes, &item.Source, &item.Scope); err != nil {
			return nil, err
		}
		if elapsed := item.LastObservedAt - item.FirstObservedAt; elapsed > 0 {
			item.AverageCPUCores = item.CPUSeconds / elapsed
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *Repository) GetActivityMetricSummary(ctx context.Context, runID, activityID string, attempt int) (*domain.ActivityMetricSummary, error) {
	if attempt < 1 {
		attempt = 1
	}
	var item domain.ActivityMetricSummary
	err := r.db.QueryRowContext(ctx, `SELECT execution_run_id, activity_id, attempt,
		COUNT(*), MIN(observed_at), MAX(observed_at),
		MAX(cpu_seconds)-MIN(cpu_seconds), MAX(memory_bytes),
		MAX(read_bytes)-MIN(read_bytes), MAX(write_bytes)-MIN(write_bytes),
		MIN(source), MIN(scope)
		FROM activity_metric_samples WHERE execution_run_id=? AND activity_id=? AND attempt=?
		GROUP BY execution_run_id, activity_id, attempt`, runID, activityID, attempt).Scan(
		&item.RunID, &item.ActivityID, &item.Attempt, &item.Samples,
		&item.FirstObservedAt, &item.LastObservedAt, &item.CPUSeconds,
		&item.PeakMemoryBytes, &item.ReadBytes, &item.WriteBytes,
		&item.Source, &item.Scope,
	)
	if err == sql.ErrNoRows || missingMetricTable(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if elapsed := item.LastObservedAt - item.FirstObservedAt; elapsed > 0 {
		item.AverageCPUCores = item.CPUSeconds / elapsed
	}
	return &item, nil
}

func (r *Repository) ListActivityMetricSamples(ctx context.Context, runID, activityID string, attempt int) ([]domain.ActivityMetricSample, error) {
	if attempt < 1 {
		attempt = 1
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_metric_samples
		WHERE execution_run_id=? AND activity_id=? AND attempt=?`, runID, activityID, attempt).Scan(&total); err != nil {
		if missingMetricTable(err) {
			return []domain.ActivityMetricSample{}, nil
		}
		return nil, err
	}
	stride := max(1, (total+1198)/1199)
	rows, err := r.db.QueryContext(ctx, `SELECT execution_run_id, activity_id, attempt,
		observed_at, cpu_seconds, memory_bytes, read_bytes, write_bytes, source, scope
		FROM (SELECT *, ROW_NUMBER() OVER (ORDER BY observed_at) AS ordinal
			FROM activity_metric_samples WHERE execution_run_id=? AND activity_id=? AND attempt=?)
		WHERE (ordinal-1) % ? = 0 OR ordinal=? ORDER BY observed_at`, runID, activityID, attempt, stride, total)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.ActivityMetricSample, 0, min(total, 1200))
	for rows.Next() {
		var item domain.ActivityMetricSample
		if err := rows.Scan(&item.RunID, &item.ActivityID, &item.Attempt,
			&item.ObservedAt, &item.CPUSeconds, &item.MemoryBytes,
			&item.ReadBytes, &item.WriteBytes, &item.Source, &item.Scope); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// Historical read-only snapshots may predate optional metric storage.
func missingMetricTable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such table: activity_metric_samples")
}
