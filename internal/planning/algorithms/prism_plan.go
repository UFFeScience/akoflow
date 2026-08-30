package algorithms

import (
	"fmt"
	"math"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type prismPredictionBreakdown struct {
	RuntimeSeconds   float64
	TransferSeconds  float64
	QueueSeconds     float64
	BootSeconds      float64
	ContainerSeconds float64
}

func compactPRISMHEFTAnchor(
	search compactPRISMContext,
) (compactPRISMState, error) {
	order, err := heftOrder(search.request)
	if err != nil {
		return compactPRISMState{}, err
	}
	state := initialCompactPRISMState(search)
	for sequence, activity := range order {
		activityOrdinal, exists := search.activityOrdinal[activity.ID]
		if !exists {
			return compactPRISMState{}, fmt.Errorf("unknown activity %q", activity.ID)
		}
		var selected *compactPRISMState
		for resourceOrdinal := range search.resources {
			if !search.feasible[activityOrdinal][resourceOrdinal] {
				continue
			}
			candidate := compactPRISMPlace(
				search,
				state,
				activityOrdinal,
				resourceOrdinal,
				sequence,
				true,
			)
			if selected == nil || compactPRISMCompleteLess(candidate, *selected, "time") {
				copy := candidate
				selected = &copy
			}
		}
		if selected == nil {
			return compactPRISMState{}, fmt.Errorf(
				"no feasible resource for activity %q",
				activity.ID,
			)
		}
		state = *selected
	}
	return state, nil
}

func compactPRISMPlan(
	id string,
	algorithm string,
	objective string,
	request domain.PlanningRequest,
	state compactPRISMState,
) domain.SchedulePlan {
	assignments := compactPRISMAssignments(state)
	for index := range assignments {
		assignments[index].ID = fmt.Sprintf(
			"%s-%s",
			id,
			assignments[index].ActivityID,
		)
		assignments[index].PlanID = id
	}
	feasible := (request.DeadlineSeconds <= 0 || state.makespan <= request.DeadlineSeconds) &&
		(request.Budget <= 0 || state.cost <= request.Budget)
	breakdown := compactPRISMPredictionBreakdown(assignments)
	confidence, confidenceScore := compactPRISMPredictionConfidence(request)
	return domain.SchedulePlan{
		ID: id, WorkflowVersionID: request.Workflow.ID,
		ExecutionScopeID:  request.ExecutionScope.ID,
		NetworkTopologyID: request.NetworkTopology.ID,
		Source:            domain.PlanningSourcePlugin, Algorithm: algorithm,
		AlgorithmVersion: "3", Objective: objective,
		DeadlineSeconds: request.DeadlineSeconds, Budget: request.Budget,
		Predicted: domain.PredictedMetrics{
			MakespanSeconds: state.makespan,
			Cost:            state.cost,
			Feasible:        feasible,
		},
		Assignments: assignments,
		Metadata: map[string]any{
			"evaluator":                 "prism-simgrid-shared-network-v2",
			"predictionConfidence":      confidence,
			"predictionConfidenceScore": confidenceScore,
			"runtimeSeconds":            breakdown.RuntimeSeconds,
			"transferSeconds":           breakdown.TransferSeconds,
			"queueSeconds":              breakdown.QueueSeconds,
			"bootOverheadSeconds":       breakdown.BootSeconds,
			"containerOverheadSeconds":  breakdown.ContainerSeconds,
			"costModel":                 "resource-active-window",
			"networkPathModel":          "simgrid-route-shared-link-events",
		},
	}
}

func compactPRISMPredictionBreakdown(assignments []domain.PlanAssignment) prismPredictionBreakdown {
	result := prismPredictionBreakdown{}
	for _, assignment := range assignments {
		result.RuntimeSeconds += assignment.PredictedRuntimeSeconds
		result.TransferSeconds += assignment.PredictedTransferSeconds
		result.QueueSeconds += prismMetadataNumber(assignment.Metadata, "queueSeconds")
		result.BootSeconds += prismMetadataNumber(assignment.Metadata, "bootOverheadSeconds")
		result.ContainerSeconds += prismMetadataNumber(assignment.Metadata, "containerOverheadSeconds")
	}
	return result
}

func compactPRISMPredictionConfidence(request domain.PlanningRequest) (string, float64) {
	if len(request.Workflow.Activities) == 0 {
		return "low", 0
	}
	profiled := make(map[string]bool)
	for _, profile := range request.ActivityProfiles {
		if profile.RuntimeSeconds > 0 {
			profiled[profile.ActivityTypeID] = true
		}
	}
	known := 0
	for _, activity := range request.Workflow.Activities {
		if activity.Simulation != nil || profiled[activity.ActivityTypeID] {
			known++
		}
	}
	score := float64(known) / float64(len(request.Workflow.Activities))
	if math.IsNaN(score) {
		score = 0
	}
	if score >= .9 {
		return "high", score
	}
	if score >= .5 {
		return "medium", score
	}
	return "low", score
}

func prismMetadataNumber(metadata map[string]any, key string) float64 {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}
