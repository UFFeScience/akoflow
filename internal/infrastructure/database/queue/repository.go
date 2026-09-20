package queue

import (
	"context"
	"database/sql"
	"encoding/json"
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

type leaseCandidate struct {
	id, category, eventType string
	payload                 []byte
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

	candidates, err := pendingLeaseCandidates(ctx, tx, now, categories)
	if err != nil {
		return nil, err
	}
	owners, err := environmentVersionOwners(ctx, tx)
	if err != nil {
		return nil, err
	}
	occupied, err := leasedExecutionEnvironments(ctx, tx, now, owners)
	if err != nil {
		return nil, err
	}
	selected := selectAdmissionCandidates(candidates, occupied, owners, limit)

	leased := make([]domainqueue.Job, 0, len(selected))
	for _, value := range selected {
		expires := now.Add(duration)
		result, err := tx.ExecContext(ctx, `UPDATE queue_jobs SET status = 'leased',
			lease_owner = ?, lease_expires_at = ?, attempts = attempts + 1,
			started_at = COALESCE(started_at, ?) WHERE id = ? AND status = 'pending'`,
			owner, expires, now, value.id)
		if err != nil {
			return nil, err
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			continue
		}
		job, err := scanJob(tx.QueryRowContext(ctx, `SELECT `+columns+` FROM queue_jobs WHERE id = ?`, value.id))
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

func pendingLeaseCandidates(ctx context.Context, tx *sql.Tx, now time.Time, categories []string) ([]leaseCandidate, error) {
	query := `SELECT id,category,event_type,payload FROM queue_jobs WHERE status='pending' AND available_at <= ?`
	args := []any{now}
	if len(categories) > 0 {
		query += ` AND category IN (` + strings.TrimRight(strings.Repeat("?,", len(categories)), ",") + `)`
		for _, category := range categories {
			args = append(args, category)
		}
	}
	rows, err := tx.QueryContext(ctx, query+` ORDER BY priority DESC,available_at,created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates := make([]leaseCandidate, 0)
	for rows.Next() {
		var value leaseCandidate
		if err := rows.Scan(&value.id, &value.category, &value.eventType, &value.payload); err != nil {
			return nil, err
		}
		candidates = append(candidates, value)
	}
	return candidates, rows.Err()
}

func selectAdmissionCandidates(candidates []leaseCandidate, occupied map[string]bool, owners map[string]string, limit int) []leaseCandidate {
	selected := make([]leaseCandidate, 0, limit)
	for _, value := range candidates {
		if len(selected) >= limit {
			break
		}
		keys, err := executionEnvironmentKeys(value.category, value.eventType, value.payload)
		if err != nil {
			// Lease malformed payloads so the handler can send them through the
			// normal retry/dead-letter path instead of leaving them pending.
			keys = nil
		}
		keys = environmentOwnerKeys(keys, owners)
		if environmentConflict(occupied, keys) {
			continue
		}
		selected = append(selected, value)
		for _, key := range keys {
			occupied[key] = true
		}
	}
	return selected
}

const executionRunRequested = "execution.run.requested"

type executionAdmissionPayload struct {
	Run struct {
		Mode string `json:"mode"`
	} `json:"run"`
	Plan struct {
		Assignments []struct {
			ResourceID string `json:"resourceId"`
		} `json:"assignments"`
	} `json:"plan"`
	Resources []struct {
		ID                   string `json:"id"`
		EnvironmentVersionID string `json:"environmentVersionId"`
	} `json:"resources"`
}

func executionEnvironmentKeys(category, eventType string, payload []byte) ([]string, error) {
	if category != domainqueue.CategoryExecution || eventType != executionRunRequested {
		return nil, nil
	}
	var request executionAdmissionPayload
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, err
	}
	if request.Run.Mode == "simulation" {
		return nil, nil
	}
	resources := make(map[string]string, len(request.Resources))
	for _, resource := range request.Resources {
		resources[resource.ID] = resource.EnvironmentVersionID
	}
	unique := make(map[string]bool)
	for _, assignment := range request.Plan.Assignments {
		if environmentID := strings.TrimSpace(resources[assignment.ResourceID]); environmentID != "" {
			unique[environmentID] = true
		}
	}
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	return keys, nil
}

func leasedExecutionEnvironments(ctx context.Context, tx *sql.Tx, now time.Time, owners map[string]string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT category,event_type,payload FROM queue_jobs
		WHERE status='leased' AND lease_expires_at > ? AND category=? AND event_type=?`,
		now, domainqueue.CategoryExecution, executionRunRequested)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	occupied := make(map[string]bool)
	for rows.Next() {
		var category, eventType string
		var payload []byte
		if err := rows.Scan(&category, &eventType, &payload); err != nil {
			return nil, err
		}
		keys, err := executionEnvironmentKeys(category, eventType, payload)
		if err != nil {
			continue
		}
		keys = environmentOwnerKeys(keys, owners)
		for _, key := range keys {
			occupied[key] = true
		}
	}
	return occupied, rows.Err()
}

func environmentVersionOwners(ctx context.Context, tx *sql.Tx) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,environment_id FROM environment_versions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	owners := make(map[string]string)
	for rows.Next() {
		var versionID, environmentID string
		if err := rows.Scan(&versionID, &environmentID); err != nil {
			return nil, err
		}
		owners[versionID] = environmentID
	}
	return owners, rows.Err()
}

func environmentOwnerKeys(versionIDs []string, owners map[string]string) []string {
	result := make([]string, 0, len(versionIDs))
	for _, versionID := range versionIDs {
		owner := owners[versionID]
		if owner == "" {
			owner = versionID
		}
		result = append(result, owner)
	}
	return result
}

func environmentConflict(occupied map[string]bool, keys []string) bool {
	for _, key := range keys {
		if occupied[key] {
			return true
		}
	}
	return false
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

func (r *Repository) CancelByAggregate(ctx context.Context, aggregateType, aggregateID string, at time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE queue_jobs SET status='cancelled', completed_at=?,
		lease_owner='', lease_expires_at=NULL
		WHERE aggregate_type=? AND aggregate_id=? AND status IN ('pending','leased')`,
		at, aggregateType, aggregateID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
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
