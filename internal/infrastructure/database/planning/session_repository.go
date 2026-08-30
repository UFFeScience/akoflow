package planning

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func encodeJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	return string(data), err
}

func (r *Repository) CreateSession(ctx context.Context, session domain.PlanningSession, runs []domain.AlgorithmRun) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	algorithms, err := encodeJSON(session.Algorithms)
	if err != nil {
		return err
	}
	configuration, err := encodeJSON(session.Configuration)
	if err != nil {
		return err
	}
	if session.Status == "" {
		session.Status = domain.PlanningStatusQueued
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now().UTC()
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO planning_sessions (
		id, workflow_version_id, execution_scope_id, network_topology_id, status, algorithms,
		progress, candidate_count, deadline_seconds, budget, configuration, failure_reason, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, session.ID, session.WorkflowVersionID,
		session.ExecutionScopeID, session.NetworkTopologyID, session.Status, algorithms, session.Progress,
		session.CandidateCount, session.DeadlineSeconds, session.Budget, configuration,
		session.FailureReason, session.CreatedAt); err != nil {
		return err
	}
	for _, run := range runs {
		persistedConfiguration := make(map[string]any, len(run.Configuration)+1)
		for key, value := range run.Configuration {
			persistedConfiguration[key] = value
		}
		persistedConfiguration["_planningEstimate"] = run.Estimate
		config, err := encodeJSON(persistedConfiguration)
		if err != nil {
			return err
		}
		if run.Status == "" {
			run.Status = domain.PlanningStatusQueued
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO planning_algorithm_runs (
			id, planning_session_id, algorithm, objective, status, progress, candidate_count, configuration, failure_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, run.ID, session.ID, run.Algorithm, run.Objective,
			run.Status, run.Progress, run.CandidateCount, config, run.FailureReason); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func scanSession(scanner interface{ Scan(...any) error }) (*domain.PlanningSession, error) {
	var session domain.PlanningSession
	var algorithms, configuration string
	var startedAt, completedAt sql.NullTime
	err := scanner.Scan(&session.ID, &session.WorkflowVersionID, &session.ExecutionScopeID,
		&session.NetworkTopologyID, &session.Status, &algorithms, &session.Progress,
		&session.CandidateCount, &session.SelectedCandidateID, &session.SelectedPlanID,
		&session.DeadlineSeconds, &session.Budget, &configuration, &session.FailureReason,
		&session.CreatedAt, &startedAt, &completedAt)
	if err != nil {
		return nil, err
	}
	if algorithms != "" {
		if err := json.Unmarshal([]byte(algorithms), &session.Algorithms); err != nil {
			return nil, err
		}
	}
	if configuration != "" {
		if err := json.Unmarshal([]byte(configuration), &session.Configuration); err != nil {
			return nil, err
		}
	}
	if startedAt.Valid {
		session.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		session.CompletedAt = &completedAt.Time
	}
	return &session, nil
}

const sessionColumns = `id, workflow_version_id, execution_scope_id, network_topology_id,
	status, algorithms, progress, candidate_count, COALESCE(selected_candidate_id, ''),
	COALESCE(selected_plan_id, ''), deadline_seconds, budget, configuration, failure_reason,
	created_at, started_at, completed_at`

func (r *Repository) FindSession(ctx context.Context, id string) (*domain.PlanningSession, error) {
	session, err := scanSession(r.db.QueryRowContext(ctx, `SELECT `+sessionColumns+` FROM planning_sessions WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return session, err
}

func (r *Repository) ListSessions(ctx context.Context) ([]domain.PlanningSession, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+sessionColumns+` FROM planning_sessions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.PlanningSession{}
	for rows.Next() {
		item, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func scanAlgorithmRun(scanner interface{ Scan(...any) error }) (*domain.AlgorithmRun, error) {
	var run domain.AlgorithmRun
	var configuration string
	var startedAt, completedAt sql.NullTime
	err := scanner.Scan(&run.ID, &run.PlanningSessionID, &run.Algorithm, &run.Objective,
		&run.Status, &run.Progress, &run.CandidateCount, &configuration, &run.FailureReason,
		&startedAt, &completedAt)
	if err != nil {
		return nil, err
	}
	if configuration != "" {
		if err := json.Unmarshal([]byte(configuration), &run.Configuration); err != nil {
			return nil, err
		}
		if raw, exists := run.Configuration["_planningEstimate"]; exists {
			encoded, err := json.Marshal(raw)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(encoded, &run.Estimate); err != nil {
				return nil, err
			}
			delete(run.Configuration, "_planningEstimate")
		}
	}
	if startedAt.Valid {
		run.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		run.CompletedAt = &completedAt.Time
	}
	return &run, nil
}

func (r *Repository) ListAlgorithmRuns(ctx context.Context, sessionID string) ([]domain.AlgorithmRun, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, planning_session_id, algorithm, objective,
		status, progress, candidate_count, configuration, failure_reason, started_at, completed_at
		FROM planning_algorithm_runs WHERE planning_session_id=? ORDER BY rowid`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.AlgorithmRun{}
	for rows.Next() {
		item, err := scanAlgorithmRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *Repository) SetSessionQueued(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE planning_sessions SET status='queued' WHERE id=? AND status='running'`, id)
	return err
}

func (r *Repository) SetSessionRunning(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE planning_sessions SET status='running', started_at=COALESCE(started_at, CURRENT_TIMESTAMP) WHERE id=? AND status IN ('queued','running')`, id)
	return err
}
func (r *Repository) SetSessionCompleted(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE planning_sessions SET status='completed', progress=1, completed_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}
func (r *Repository) SetSessionFailed(ctx context.Context, id, reason string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE planning_sessions SET status='failed', failure_reason=?, completed_at=CURRENT_TIMESTAMP WHERE id=?`, reason, id)
	return err
}
func (r *Repository) SetSessionCancelled(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE planning_sessions SET status='cancelled', completed_at=CURRENT_TIMESTAMP WHERE id=? AND status IN ('queued','running')`, id)
	return err
}
func (r *Repository) SetAlgorithmRunRunning(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE planning_algorithm_runs SET status='running', started_at=COALESCE(started_at,CURRENT_TIMESTAMP) WHERE id=?`, id)
	return err
}
func (r *Repository) SetAlgorithmRunEstimate(ctx context.Context, id string, estimate domain.PlanningEstimate) error {
	var encodedConfiguration string
	if err := r.db.QueryRowContext(ctx, `SELECT configuration FROM planning_algorithm_runs WHERE id=?`, id).Scan(&encodedConfiguration); err != nil {
		return err
	}
	configuration := map[string]any{}
	if encodedConfiguration != "" {
		if err := json.Unmarshal([]byte(encodedConfiguration), &configuration); err != nil {
			return err
		}
	}
	configuration["_planningEstimate"] = estimate
	encoded, err := json.Marshal(configuration)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE planning_algorithm_runs SET configuration=? WHERE id=?`, string(encoded), id)
	return err
}
func (r *Repository) UpdateAlgorithmRunProgress(ctx context.Context, id string, progress float64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE planning_algorithm_runs SET progress=? WHERE id=?`, progress, id)
	return err
}
func (r *Repository) UpdateSessionProgress(ctx context.Context, id string, progress float64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE planning_sessions SET progress=? WHERE id=?`, progress, id)
	return err
}
func (r *Repository) SetAlgorithmRunCompleted(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE planning_algorithm_runs SET status='completed', progress=1, completed_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}
func (r *Repository) SetAlgorithmRunFailed(ctx context.Context, id, reason string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE planning_algorithm_runs SET status='failed', failure_reason=?, completed_at=CURRENT_TIMESTAMP WHERE id=?`, reason, id)
	return err
}
func (r *Repository) SetAlgorithmRunCancelled(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE planning_algorithm_runs SET status='cancelled', completed_at=CURRENT_TIMESTAMP WHERE id=? AND status IN ('queued','running')`, id)
	return err
}

func (r *Repository) SaveCandidate(ctx context.Context, candidate domain.PlanCandidate) error {
	plan, err := encodeJSON(candidate.Plan)
	if err != nil {
		return err
	}
	if candidate.CreatedAt.IsZero() {
		candidate.CreatedAt = time.Now().UTC()
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO planning_candidates (
		id, planning_session_id, algorithm_run_id, algorithm, objective, rank, pareto_optimal,
		dominated, feasible, predicted_makespan_seconds, predicted_cost, plan, fingerprint, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT DO NOTHING`, candidate.ID, candidate.PlanningSessionID,
		candidate.AlgorithmRunID, candidate.Algorithm, candidate.Objective, candidate.Rank,
		candidate.ParetoOptimal, candidate.Dominated, candidate.Feasible,
		candidate.Predicted.MakespanSeconds, candidate.Predicted.Cost, plan,
		candidate.Fingerprint, candidate.CreatedAt)
	if err != nil {
		return err
	}
	inserted, _ := result.RowsAffected()
	if inserted > 0 {
		_, err = r.db.ExecContext(ctx, `UPDATE planning_algorithm_runs SET candidate_count=candidate_count+1 WHERE id=?`, candidate.AlgorithmRunID)
		if err != nil {
			return err
		}
		_, err = r.db.ExecContext(ctx, `UPDATE planning_sessions SET candidate_count=candidate_count+1 WHERE id=?`, candidate.PlanningSessionID)
	}
	return err
}

func scanCandidate(scanner interface{ Scan(...any) error }) (*domain.PlanCandidate, error) {
	var candidate domain.PlanCandidate
	var plan string
	err := scanner.Scan(&candidate.ID, &candidate.PlanningSessionID, &candidate.AlgorithmRunID,
		&candidate.Algorithm, &candidate.Objective, &candidate.Rank, &candidate.ParetoOptimal,
		&candidate.Dominated, &candidate.Feasible, &candidate.Predicted.MakespanSeconds,
		&candidate.Predicted.Cost, &plan, &candidate.Fingerprint, &candidate.CreatedAt)
	if err != nil {
		return nil, err
	}
	candidate.Predicted.Feasible = candidate.Feasible
	if err := json.Unmarshal([]byte(plan), &candidate.Plan); err != nil {
		return nil, err
	}
	return &candidate, nil
}

const candidateColumns = `id, planning_session_id, algorithm_run_id, algorithm, objective,
	rank, pareto_optimal, dominated, feasible, predicted_makespan_seconds, predicted_cost,
	plan, fingerprint, created_at`

func (r *Repository) ListCandidates(ctx context.Context, sessionID string) ([]domain.PlanCandidate, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+candidateColumns+` FROM planning_candidates WHERE planning_session_id=? ORDER BY rank, predicted_makespan_seconds, predicted_cost`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.PlanCandidate{}
	for rows.Next() {
		item, err := scanCandidate(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func scanCandidateSummary(scanner interface{ Scan(...any) error }) (*domain.PlanCandidate, error) {
	var candidate domain.PlanCandidate
	err := scanner.Scan(
		&candidate.ID,
		&candidate.PlanningSessionID,
		&candidate.AlgorithmRunID,
		&candidate.Algorithm,
		&candidate.Objective,
		&candidate.Rank,
		&candidate.ParetoOptimal,
		&candidate.Dominated,
		&candidate.Feasible,
		&candidate.Predicted.MakespanSeconds,
		&candidate.Predicted.Cost,
		&candidate.Plan.AssignmentCount,
		&candidate.Fingerprint,
		&candidate.CreatedAt,
	)
	candidate.Predicted.Feasible = candidate.Feasible
	return &candidate, err
}

func (r *Repository) ListCandidateSummaries(ctx context.Context, sessionID string) ([]domain.PlanCandidate, error) {
	const columns = `id, planning_session_id, algorithm_run_id, algorithm, objective,
		rank, pareto_optimal, dominated, feasible, predicted_makespan_seconds, predicted_cost,
		COALESCE(json_array_length(json_extract(plan, '$.assignments')), 0), fingerprint, created_at`
	rows, err := r.db.QueryContext(ctx, `SELECT `+columns+` FROM planning_candidates
		WHERE planning_session_id=? ORDER BY rank, predicted_makespan_seconds, predicted_cost`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.PlanCandidate{}
	for rows.Next() {
		item, err := scanCandidateSummary(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *Repository) FindCandidate(ctx context.Context, id string) (*domain.PlanCandidate, error) {
	item, err := scanCandidate(r.db.QueryRowContext(ctx, `SELECT `+candidateColumns+` FROM planning_candidates WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (r *Repository) UpdateCandidateRanks(ctx context.Context, candidates []domain.PlanCandidate) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range candidates {
		if _, err := tx.ExecContext(ctx, `UPDATE planning_candidates SET rank=?, pareto_optimal=?, dominated=? WHERE id=?`, item.Rank, item.ParetoOptimal, item.Dominated, item.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) SelectCandidate(ctx context.Context, sessionID string, candidate domain.PlanCandidate) error {
	if candidate.PlanningSessionID != sessionID {
		return fmt.Errorf("candidate does not belong to planning session")
	}
	if existing, err := r.Find(ctx, candidate.Plan.ID); err != nil {
		return err
	} else if existing == nil {
		if err := r.Save(ctx, candidate.Plan); err != nil {
			return err
		}
	}
	_, err := r.db.ExecContext(ctx, `UPDATE planning_sessions SET selected_candidate_id=?, selected_plan_id=? WHERE id=?`, candidate.ID, candidate.Plan.ID, sessionID)
	return err
}
