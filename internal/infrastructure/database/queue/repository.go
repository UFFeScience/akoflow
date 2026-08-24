package queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

type Repository struct {
	db    *sql.DB
	owned bool
}

func New(db *sql.DB) (*Repository, error) {
	if db == nil {
		return nil, errors.New("queue repository requires a database")
	}
	return &Repository{db: db}, nil
}

func (r *Repository) Close() error {
	if r.owned {
		return r.db.Close()
	}
	return nil
}

func (r *Repository) Publish(ctx context.Context, job domainqueue.Job) (domainqueue.Job, error) {
	if err := job.Validate(); err != nil {
		return domainqueue.Job{}, err
	}
	if job.AvailableAt.IsZero() {
		job.AvailableAt = time.Now().UTC()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = job.AvailableAt
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO queue_jobs (
		id, category, event_type, aggregate_type, aggregate_id, payload, status,
		priority, available_at, attempts, max_attempts, idempotency_key, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)
	ON CONFLICT(idempotency_key) WHERE idempotency_key IS NOT NULL AND idempotency_key <> ''
	DO NOTHING`, job.ID, job.Category, job.Type, job.AggregateType, job.AggregateID,
		job.Payload, job.Status, job.Priority, job.AvailableAt, job.Attempts,
		job.MaxAttempts, job.IdempotencyKey, job.CreatedAt)
	if err != nil {
		return domainqueue.Job{}, fmt.Errorf("publish queue job: %w", err)
	}
	if job.IdempotencyKey != "" {
		var existingID string
		if err := r.db.QueryRowContext(ctx, `SELECT id FROM queue_jobs WHERE idempotency_key = ?`, job.IdempotencyKey).Scan(&existingID); err != nil {
			return domainqueue.Job{}, err
		}
		existing, err := r.FindByID(ctx, existingID)
		if err != nil {
			return domainqueue.Job{}, err
		}
		if existing == nil {
			return domainqueue.Job{}, errors.New("idempotent queue job disappeared")
		}
		return *existing, nil
	}
	return job, nil
}

func (r *Repository) Lease(ctx context.Context, owner string, categories []string, limit int, duration time.Duration) ([]domainqueue.Job, error) {
	if owner == "" || limit < 1 || duration <= 0 {
		return nil, errors.New("lease owner, positive limit and duration are required")
	}
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query := `SELECT id FROM queue_jobs WHERE status = 'pending' AND available_at <= ?`
	args := []any{now}
	if len(categories) > 0 {
		query += ` AND category IN (` + strings.TrimRight(strings.Repeat("?,", len(categories)), ",") + `)`
		for _, category := range categories {
			args = append(args, category)
		}
	}
	query += ` ORDER BY priority DESC, available_at, created_at LIMIT ?`
	args = append(args, limit)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	leased := make([]domainqueue.Job, 0, len(ids))
	for _, id := range ids {
		expires := now.Add(duration)
		result, err := tx.ExecContext(ctx, `UPDATE queue_jobs SET status = 'leased',
			lease_owner = ?, lease_expires_at = ?, attempts = attempts + 1,
			started_at = COALESCE(started_at, ?) WHERE id = ? AND status = 'pending'`,
			owner, expires, now, id)
		if err != nil {
			return nil, err
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			continue
		}
		job, err := scanJob(tx.QueryRowContext(ctx, `SELECT `+columns+` FROM queue_jobs WHERE id = ?`, id))
		if err != nil {
			return nil, err
		}
		leased = append(leased, *job)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return leased, nil
}

func (r *Repository) Complete(ctx context.Context, id, owner string, at time.Time) error {
	return r.transition(ctx, `UPDATE queue_jobs SET status='completed', completed_at=?, lease_owner='', lease_expires_at=NULL
		WHERE id=? AND status='leased' AND lease_owner=?`, at, id, owner)
}

func (r *Repository) RenewLease(ctx context.Context, id, owner string, expiresAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE queue_jobs SET lease_expires_at=?
		WHERE id=? AND status='leased' AND lease_owner=?`, expiresAt, id, owner)
	if err != nil {
		return err
	}
	return requireOne(result, "renew queue job lease")
}

func (r *Repository) Retry(ctx context.Context, id, owner string, cause error, availableAt time.Time) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	result, err := r.db.ExecContext(ctx, `UPDATE queue_jobs SET
		status=CASE WHEN attempts >= max_attempts THEN 'failed' ELSE 'pending' END,
		available_at=?, last_error=?, lease_owner='', lease_expires_at=NULL,
		completed_at=CASE WHEN attempts >= max_attempts THEN ? ELSE NULL END
		WHERE id=? AND status='leased' AND lease_owner=?`, availableAt, message, availableAt, id, owner)
	if err != nil {
		return err
	}
	return requireOne(result, "retry queue job")
}

func (r *Repository) Cancel(ctx context.Context, id string, at time.Time) error {
	return r.transition(ctx, `UPDATE queue_jobs SET status='cancelled', completed_at=?, lease_owner='', lease_expires_at=NULL
		WHERE id=? AND status IN ('pending','leased')`, at, id)
}

func (r *Repository) ReleaseExpired(ctx context.Context, now time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE queue_jobs SET status='pending', lease_owner='', lease_expires_at=NULL,
		available_at=? WHERE status='leased' AND lease_expires_at <= ?`, now, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *Repository) transition(ctx context.Context, query string, args ...any) error {
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	return requireOne(result, "transition queue job")
}

func requireOne(result sql.Result, operation string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("%s affected %d rows", operation, count)
	}
	return nil
}

const columns = `id, category, event_type, aggregate_type, aggregate_id, payload,
	status, priority, available_at, lease_owner, lease_expires_at, attempts,
	max_attempts, COALESCE(idempotency_key,''), last_error, created_at, started_at, completed_at`

func scanJob(row interface{ Scan(...any) error }) (*domainqueue.Job, error) {
	var job domainqueue.Job
	var status string
	var lease, started, completed sql.NullTime
	if err := row.Scan(&job.ID, &job.Category, &job.Type, &job.AggregateType,
		&job.AggregateID, &job.Payload, &status, &job.Priority, &job.AvailableAt,
		&job.LeaseOwner, &lease, &job.Attempts, &job.MaxAttempts,
		&job.IdempotencyKey, &job.LastError, &job.CreatedAt, &started, &completed); err != nil {
		return nil, err
	}
	job.Status = domainqueue.Status(status)
	if lease.Valid {
		job.LeaseExpiresAt = &lease.Time
	}
	if started.Valid {
		job.StartedAt = &started.Time
	}
	if completed.Valid {
		job.CompletedAt = &completed.Time
	}
	return &job, nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (*domainqueue.Job, error) {
	job, err := scanJob(r.db.QueryRowContext(ctx, `SELECT `+columns+` FROM queue_jobs WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return job, err
}
