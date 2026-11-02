package algorithms

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/storagebudget"
)

type PRISM struct{ Objective string }

func NewPRISMTime() PRISM { return PRISM{Objective: "time"} }
func NewPRISMCost() PRISM { return PRISM{Objective: "cost"} }

func (p PRISM) Descriptor() ports.SchedulerDescriptor {
	id, name := "prism-time", "PRISM Time"
	description := "PRISM-CC options ranked by predicted makespan, including an eligible native HEFT baseline."
	if p.Objective == "cost" {
		id, name = "prism-cost", "PRISM Cost"
		description = "PRISM-CC options ranked exclusively by predicted cost."
	}
	return ports.SchedulerDescriptor{
		ID: id, Name: name, Origin: "native", Exclusive: true, Objective: p.Objective, Multiple: true,
		Description: description,
		Defaults: map[string]any{
			"beamWidth": 120, "optionCount": 25, "readyBranchLimit": 3,
			"interferenceAware": true,
			"slaAware":          true,
		},
	}
}

func (p PRISM) Schedule(
	ctx context.Context,
	request domain.PlanningRequest,
	configuration map[string]any,
	progress ports.ProgressReporter,
	sink ports.CandidateSink,
) error {
	searchRequest := request
	selectionRequest := request
	slaAware := true
	if raw, exists := configuration["slaAware"]; exists {
		value, valid := raw.(bool)
		if !valid {
			return fmt.Errorf("slaAware must be a boolean")
		}
		slaAware = value
	}
	if !slaAware {
		// Ablate only SLA-guided pruning and final filtering. Keep the original
		// limits on emitted plans so their predicted feasibility stays honest.
		searchRequest.DeadlineSeconds, searchRequest.Budget = 0, 0
		selectionRequest.DeadlineSeconds, selectionRequest.Budget = 0, 0
	}
	if request.InterferenceKnowledge != nil {
		searchRequest.Interference = request.InterferenceKnowledge
	}
	if aware, exists := configuration["interferenceAware"].(bool); exists && !aware {
		// Keep the immutable session matrix on the candidate sink so execution
		// still suffers interference, but hide it from this planner run.
		searchRequest.Interference = nil
	}
	search, err := newCompactPRISMContext(searchRequest, configuration)
	if err != nil {
		return err
	}
	states, err := runCompactPRISMSearch(ctx, search, p.Objective, progress)
	if err != nil {
		return err
	}
	// Equal placement signatures produce the same detailed event simulation.
	// Remove them before the expensive shared-network evaluation instead of
	// evaluating every duplicate retained by the beam.
	states = dedupeCompactPRISMStates(states, p.Objective)
	states, err = reevaluateCompleteCompactPRISMStates(search, states)
	if err != nil {
		return fmt.Errorf("evaluate PRISM candidates with shared network: %w", err)
	}
	baselineIncluded := false
	baselineMakespan := 0.0
	baselineUnavailable := ""
	if p.Objective == "time" {
		baseline, baselineErr := compactPRISMHEFTAnchor(ctx, search)
		if baselineErr != nil {
			var quota *storagebudget.InsufficientQuota
			if !errors.Is(baselineErr, errPRISMHEFTBaselineIneligible) && !errors.As(baselineErr, &quota) {
				return fmt.Errorf("evaluate native HEFT baseline: %w", baselineErr)
			}
			baselineUnavailable = baselineErr.Error()
		} else {
			states = append(states, baseline)
			baselineIncluded = true
			baselineMakespan = baseline.makespan
		}
	}
	optionCount := intOption(configuration, "optionCount", 25, 1, 1000)
	states = selectCompleteCompactPRISMOptions(
		states,
		optionCount,
		p.Objective,
		selectionRequest,
	)
	for index, state := range states {
		id := fmt.Sprintf("%s-candidate-%d", p.Descriptor().ID, index+1)
		plan := compactPRISMPlan(id, p.Descriptor().ID, p.Objective, request, search, state)
		plan.Metadata["slaAware"] = slaAware
		if p.Objective == "time" {
			plan.AlgorithmVersion = "4"
			origin := state.candidateOrigin
			if origin == "" {
				origin = "prism-search"
			}
			plan.Metadata["candidateOrigin"] = origin
			plan.Metadata["selectionPolicy"] = "minimum common evaluation including eligible native HEFT baseline"
			plan.Metadata["heftBaselineIncluded"] = baselineIncluded
			if baselineIncluded {
				plan.Metadata["heftBaselineEvaluatedMakespanSeconds"] = baselineMakespan
			}
			if baselineUnavailable != "" {
				plan.Metadata["heftBaselineUnavailable"] = baselineUnavailable
			}
		}
		if err := sink.Emit(ctx, plan); err != nil {
			return err
		}
	}
	return nil
}

func prismCommunicationRanks(
	request domain.PlanningRequest,
	topological []domain.Activity,
	resources []domain.Resource,
) (map[string]float64, error) {
	return prismCommunicationRanksWithRouter(
		request,
		topological,
		resources,
		newCompactPRISMRouter(request.NetworkTopology, resources),
	)
}

func prismCommunicationRanksWithRouter(
	request domain.PlanningRequest,
	topological []domain.Activity,
	resources []domain.Resource,
	router compactPRISMRouter,
) (map[string]float64, error) {
	byID := map[string]domain.Activity{}
	successors := map[string][]string{}
	for _, activity := range topological {
		byID[activity.ID] = activity
	}
	for _, dependency := range request.Workflow.Dependencies {
		successors[dependency.DependsOnActivityID] = append(
			successors[dependency.DependsOnActivityID],
			dependency.ActivityID,
		)
	}
	bytes := dataBytes(request.Workflow)
	ranks := map[string]float64{}
	visiting := map[string]bool{}
	var rank func(string) (float64, error)
	rank = func(id string) (float64, error) {
		if value, exists := ranks[id]; exists {
			return value, nil
		}
		if visiting[id] {
			return 0, fmt.Errorf("workflow contains a cycle at activity %q", id)
		}
		visiting[id] = true
		computation := 0.0
		for _, resource := range resources {
			computation += duration(byID[id], resource, request.ActivityProfiles)
		}
		computation /= float64(len(resources))
		longestSuccessor := 0.0
		for _, successor := range successors[id] {
			successorRank, err := rank(successor)
			if err != nil {
				return 0, err
			}
			communication := averageTransferSecondsWithRouter(
				router,
				resources,
				bytes[successor][id],
			)
			longestSuccessor = math.Max(longestSuccessor, communication+successorRank)
		}
		visiting[id] = false
		ranks[id] = computation + longestSuccessor
		return ranks[id], nil
	}
	for _, activity := range topological {
		if _, err := rank(activity.ID); err != nil {
			return nil, err
		}
	}
	return ranks, nil
}

func averageTransferSeconds(
	topology domain.NetworkTopology,
	resources []domain.Resource,
	bytes int64,
) float64 {
	return averageTransferSecondsWithRouter(
		newCompactPRISMRouter(topology, resources),
		resources,
		bytes,
	)
}

func averageTransferSecondsWithRouter(
	router compactPRISMRouter,
	resources []domain.Resource,
	bytes int64,
) float64 {
	if bytes <= 0 || len(resources) < 2 {
		return 0
	}
	total, pairs := 0.0, 0
	for _, source := range resources {
		for _, target := range resources {
			if source.ID == target.ID {
				continue
			}
			route, exists := router.route(source.ID, target.ID)
			if !exists {
				continue
			}
			seconds := compactPRISMTransferSecondsOnRoute(
				compactPRISMState{},
				source.ID,
				target.ID,
				bytes,
				0,
				route,
			)
			if math.IsInf(seconds, 1) {
				continue
			}
			total += seconds
			pairs++
		}
	}
	if pairs == 0 {
		return 0
	}
	return total / float64(pairs)
}

func minimumActivityCost(
	activity domain.Activity,
	resources []domain.Resource,
	profiles []domain.ActivityResourceProfile,
) float64 {
	minimum := math.Inf(1)
	for _, resource := range resources {
		if resourceFeasible(activity, resource) {
			minimum = math.Min(
				minimum,
				duration(activity, resource, profiles)*resource.PricePerSecond,
			)
		}
	}
	if math.IsInf(minimum, 1) {
		return 0
	}
	return minimum
}

func selectCompleteCompactPRISMOptions(
	states []compactPRISMState,
	limit int,
	objective string,
	request domain.PlanningRequest,
) []compactPRISMState {
	states = dedupeCompactPRISMStates(states, objective)
	eligible := make([]compactPRISMState, 0, len(states))
	for _, state := range states {
		if !compactPRISMWithinConstraints(state, request) {
			continue
		}
		eligible = append(eligible, state)
	}
	if len(eligible) == 0 {
		// Preserve the best plans found even when every complete option misses
		// the configured SLA. compactPRISMPlan marks them as infeasible.
		eligible = append(eligible, states...)
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		return compactPRISMCompleteLess(eligible[i], eligible[j], objective)
	})
	if len(eligible) > limit {
		eligible = eligible[:limit]
	}
	return eligible
}
