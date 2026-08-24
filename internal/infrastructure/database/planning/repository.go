package planning

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type Repository struct{ db *sql.DB }

var _ ports.PlanStore = (*Repository)(nil)

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Save(ctx context.Context, plan domain.SchedulePlan) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	configuration, err := json.Marshal(plan.Metadata)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO schedule_plans (
		id, workflow_version_id, execution_scope_id, network_topology_id, source, algorithm,
		algorithm_version, objective, status, deadline_seconds, budget,
		predicted_makespan_seconds, predicted_cost, predicted_feasible, configuration
	) VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, 'valid', ?, ?, ?, ?, ?, ?)`, plan.ID,
		plan.WorkflowVersionID, plan.ExecutionScopeID, plan.NetworkTopologyID, plan.Source, plan.Algorithm,
		plan.AlgorithmVersion, plan.Objective, plan.DeadlineSeconds, plan.Budget,
		plan.Predicted.MakespanSeconds, plan.Predicted.Cost, plan.Predicted.Feasible,
		string(configuration)); err != nil {
		return err
	}
	for _, assignment := range plan.Assignments {
		metadata, err := json.Marshal(assignment.Metadata)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO schedule_plan_assignments (
			id, schedule_plan_id, activity_id, resource_id, core_id, slot_id,
			order_on_resource, priority, predicted_ready_at, predicted_start_at,
			predicted_finish_at, predicted_runtime_seconds, predicted_transfer_seconds,
			predicted_cost, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, assignment.ID,
			plan.ID, assignment.ActivityID, assignment.ResourceID, assignment.CoreID,
			assignment.SlotID, assignment.OrderOnResource, assignment.Priority,
			assignment.PredictedReadyAt, assignment.PredictedStartAt,
			assignment.PredictedFinishAt, assignment.PredictedRuntimeSeconds,
			assignment.PredictedTransferSeconds, assignment.PredictedCost, string(metadata)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) Find(ctx context.Context, id string) (*domain.SchedulePlan, error) {
	var plan domain.SchedulePlan
	var source, configuration string
	err := r.db.QueryRowContext(ctx, `SELECT id, workflow_version_id, execution_scope_id,
		COALESCE(network_topology_id, ''), source, algorithm, algorithm_version, objective, deadline_seconds, budget,
		predicted_makespan_seconds, predicted_cost, predicted_feasible, configuration
		FROM schedule_plans WHERE id = ?`, id).Scan(&plan.ID, &plan.WorkflowVersionID,
		&plan.ExecutionScopeID, &plan.NetworkTopologyID, &source, &plan.Algorithm, &plan.AlgorithmVersion,
		&plan.Objective, &plan.DeadlineSeconds, &plan.Budget,
		&plan.Predicted.MakespanSeconds, &plan.Predicted.Cost, &plan.Predicted.Feasible,
		&configuration)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	plan.Source = domain.PlanningSource(source)
	if configuration != "" {
		if err := json.Unmarshal([]byte(configuration), &plan.Metadata); err != nil {
			return nil, err
		}
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, schedule_plan_id, activity_id, resource_id,
		core_id, slot_id, order_on_resource, priority, predicted_ready_at,
		predicted_start_at, predicted_finish_at, predicted_runtime_seconds,
		predicted_transfer_seconds, predicted_cost, metadata
		FROM schedule_plan_assignments WHERE schedule_plan_id = ?
		ORDER BY resource_id, core_id, slot_id, order_on_resource`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var assignment domain.PlanAssignment
		var metadata string
		if err := rows.Scan(&assignment.ID, &assignment.PlanID, &assignment.ActivityID,
			&assignment.ResourceID, &assignment.CoreID, &assignment.SlotID,
			&assignment.OrderOnResource, &assignment.Priority, &assignment.PredictedReadyAt,
			&assignment.PredictedStartAt, &assignment.PredictedFinishAt,
			&assignment.PredictedRuntimeSeconds, &assignment.PredictedTransferSeconds,
			&assignment.PredictedCost, &metadata); err != nil {
			return nil, err
		}
		if metadata != "" {
			if err := json.Unmarshal([]byte(metadata), &assignment.Metadata); err != nil {
				return nil, err
			}
		}
		plan.Assignments = append(plan.Assignments, assignment)
	}
	return &plan, rows.Err()
}

func (r *Repository) List(ctx context.Context) ([]domain.SchedulePlan, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, workflow_version_id, execution_scope_id,
		COALESCE(network_topology_id, ''), source, algorithm, algorithm_version, objective,
		deadline_seconds, budget, predicted_makespan_seconds, predicted_cost, predicted_feasible,
		(SELECT COUNT(*) FROM schedule_plan_assignments a WHERE a.schedule_plan_id = schedule_plans.id)
		FROM schedule_plans ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plans := make([]domain.SchedulePlan, 0)
	for rows.Next() {
		var plan domain.SchedulePlan
		var source string
		if err := rows.Scan(&plan.ID, &plan.WorkflowVersionID, &plan.ExecutionScopeID,
			&plan.NetworkTopologyID, &source, &plan.Algorithm, &plan.AlgorithmVersion,
			&plan.Objective, &plan.DeadlineSeconds, &plan.Budget,
			&plan.Predicted.MakespanSeconds, &plan.Predicted.Cost, &plan.Predicted.Feasible,
			&plan.AssignmentCount); err != nil {
			return nil, err
		}
		plan.Source = domain.PlanningSource(source)
		plans = append(plans, plan)
	}
	return plans, rows.Err()
}
