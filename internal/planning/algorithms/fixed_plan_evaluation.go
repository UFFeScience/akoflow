package algorithms

import (
	"fmt"
	"github.com/UFFeScience/akoflow/internal/domain"
)

// EvaluateFixedPlan uses the production PRISM stage evaluator without changing
// resource/core placement or order. It prevents stale predicted metrics after
// updating calibration parameters for an offline validation.
func EvaluateFixedPlan(request domain.PlanningRequest, plan domain.SchedulePlan) (domain.SchedulePlan, error) {
	search, err := newCompactPRISMContext(request, map[string]any{})
	if err != nil {
		return plan, err
	}
	if len(plan.Assignments) != len(search.activities) {
		return plan, fmt.Errorf("fixed evaluation requires complete placement")
	}
	seen := map[string]bool{}
	for _, a := range plan.Assignments {
		ordinal, ok := search.activityOrdinal[a.ActivityID]
		if !ok || seen[a.ActivityID] {
			return plan, fmt.Errorf("unknown or duplicate fixed activity")
		}
		seen[a.ActivityID] = true
		r := compactPRISMResourceOrdinal(search, a.ResourceID)
		if r < 0 || r >= len(search.resources) || !search.feasible[ordinal][r] {
			return plan, fmt.Errorf("ineligible fixed resource for %s", a.ActivityID)
		}
		coreOK := false
		for _, core := range search.resources[r].cores {
			if core == a.CoreID {
				coreOK = true
			}
		}
		if !coreOK {
			return plan, fmt.Errorf("invalid fixed core for %s", a.ActivityID)
		}
	}
	state := initialCompactPRISMState(search)
	state.evaluatedAssignments = append([]domain.PlanAssignment(nil), plan.Assignments...)
	state, err = evaluateCompleteCompactPRISMState(search, state)
	if err != nil {
		return plan, err
	}
	plan.Assignments = state.evaluatedAssignments
	plan.Predicted.MakespanSeconds = state.makespan
	plan.Predicted.Cost = state.cost
	plan.Predicted.Feasible = true
	if request.DeadlineSeconds > 0 && state.makespan > request.DeadlineSeconds {
		plan.Predicted.Feasible = false
	}
	if request.Budget > 0 && state.cost > request.Budget {
		plan.Predicted.Feasible = false
	}
	metadata := clonePRISMEvaluationMetadata(plan.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["evaluationMode"] = "fixed-placement-production-stage-evaluator"
	plan.Metadata = metadata
	return plan, nil
}
