package algorithms

import (
	"context"
	"errors"
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

var errPRISMHEFTBaselineIneligible = errors.New("native HEFT placement violates PRISM placement eligibility")

type prismHEFTSink struct{ plans []domain.SchedulePlan }

func (s *prismHEFTSink) Emit(_ context.Context, plan domain.SchedulePlan) error {
	s.plans = append(s.plans, plan)
	return nil
}

// Use the actual native insertion HEFT output, not a HEFT-like greedy PRISM
// placement. Keep it out of beam pruning and evaluate it with the same final
// model as the surviving PRISM candidates.
func compactPRISMHEFTAnchor(ctx context.Context, search compactPRISMContext) (compactPRISMState, error) {
	sink := &prismHEFTSink{}
	if err := (HEFT{}).Schedule(ctx, search.heftRequest, nil, nil, sink); err != nil {
		return compactPRISMState{}, err
	}
	if len(sink.plans) != 1 {
		return compactPRISMState{}, fmt.Errorf("native HEFT emitted %d plans", len(sink.plans))
	}
	state := compactPRISMState{
		evaluatedAssignments: sink.plans[0].Assignments,
		candidateOrigin:      "heft-baseline",
		signature:            extendScheduleSignature(14695981039346656037, "native-heft-baseline"),
	}
	for _, a := range state.evaluatedAssignments {
		activity, ok := search.activityOrdinal[a.ActivityID]
		if !ok {
			return compactPRISMState{}, errPRISMHEFTBaselineIneligible
		}
		resource := compactPRISMResourceOrdinal(search, a.ResourceID)
		if resource < 0 || resource >= len(search.resources) || search.resources[resource].resource.ID != a.ResourceID || !search.feasible[activity][resource] {
			return compactPRISMState{}, errPRISMHEFTBaselineIneligible
		}
		state.signature = extendScheduleSignature(state.signature, a.ActivityID, a.ResourceID, a.CoreID, a.SlotID, fmt.Sprint(a.OrderOnResource))
	}
	return evaluateCompleteCompactPRISMState(search, state)
}

func compactPRISMPlan(
	id string,
	algorithm string,
	objective string,
	request domain.PlanningRequest,
	search compactPRISMContext,
	state compactPRISMState,
) domain.SchedulePlan {
	assignments := compactPRISMAssignments(search, state)
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
	interferenceModel := "none"
	if prismInterferencePresent(search.request.Interference) {
		interferenceModel = search.request.Interference.Model + "-" + search.request.Interference.Aggregation
	}
	plan := domain.SchedulePlan{
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
			"interferenceModel":         interferenceModel,
			"interferenceAware":         prismInterferencePresent(search.request.Interference),
			"executionInterference":     prismInterferencePresent(request.Interference),
		},
	}
	addCloudLifecycle(&plan, request)
	return plan
}

func prismInterferencePresent(matrix *domain.InterferenceMatrix) bool {
	return matrix != nil && (len(matrix.Entries) > 0 || len(matrix.Rules) > 0 || len(matrix.Groups) > 0)
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
