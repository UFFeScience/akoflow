package algorithms

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type PRISM struct{ Objective string }

func NewPRISMTime() PRISM { return PRISM{Objective: "time"} }
func NewPRISMCost() PRISM { return PRISM{Objective: "cost"} }

func (p PRISM) Descriptor() ports.SchedulerDescriptor {
	id, name := "prism-time", "PRISM Time"
	description := "PRISM-CC options ranked exclusively by predicted makespan."
	if p.Objective == "cost" {
		id, name = "prism-cost", "PRISM Cost"
		description = "PRISM-CC options ranked exclusively by predicted cost."
	}
	return ports.SchedulerDescriptor{
		ID: id, Name: name, Objective: p.Objective, Multiple: true,
		Description: description,
		Defaults: map[string]any{
			"beamWidth": 120, "optionCount": 25, "readyBranchLimit": 3,
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
	search, err := newCompactPRISMContext(request, configuration)
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
	optionCount := intOption(configuration, "optionCount", 25, 1, 1000)
	states = selectCompleteCompactPRISMOptions(
		states,
		optionCount,
		p.Objective,
		request,
	)
	for index, state := range states {
		id := fmt.Sprintf("%s-candidate-%d", p.Descriptor().ID, index+1)
		plan := compactPRISMPlan(id, p.Descriptor().ID, p.Objective, request, search, state)
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
