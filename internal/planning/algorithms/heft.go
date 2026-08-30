package algorithms

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type HEFT struct{}

type heftResource struct {
	resource domain.Resource
	cores    []string
}

type heftPlacement struct {
	resource domain.Resource
	core     string
	readyAt  float64
	transfer float64
	runtime  float64
	start    float64
	finish   float64
	makespan float64
	cost     float64
}

func (HEFT) Descriptor() ports.SchedulerDescriptor {
	return ports.SchedulerDescriptor{ID: "heft", Name: "HEFT", Objective: "time", Multiple: false, Description: "Classic heterogeneous earliest-finish-time baseline."}
}

func (HEFT) Schedule(ctx context.Context, request domain.PlanningRequest, _ map[string]any, progress ports.ProgressReporter, sink ports.CandidateSink) error {
	order, err := heftOrder(request)
	if err != nil {
		return err
	}
	resources := schedulableResources(request)
	if len(resources) == 0 {
		return fmt.Errorf("execution scope has no schedulable resources")
	}
	candidates := make([]heftResource, 0, len(resources))
	for _, resource := range resources {
		candidates = append(candidates, heftResource{
			resource: resource,
			cores:    cores(resource),
		})
	}
	preds := predecessors(request.Workflow)
	bytes := dataBytes(request.Workflow)
	state := initialState()
	for index, activity := range order {
		if err := ctx.Err(); err != nil {
			return err
		}
		var selected *heftPlacement
		for _, candidate := range candidates {
			resource := candidate.resource
			if !resourceFeasible(activity, resource) {
				continue
			}
			readyAt, transfer := heftReadyAt(
				request.NetworkTopology,
				state,
				resource.ID,
				preds[activity.ID],
				bytes[activity.ID],
			)
			runtime := duration(activity, resource, request.ActivityProfiles)
			for _, core := range candidate.cores {
				placement := evaluateHEFTPlacement(
					state,
					resource,
					core,
					readyAt,
					transfer,
					runtime,
				)
				if betterHEFTPlacement(placement, selected) {
					selected = &placement
				}
			}
		}
		if selected == nil {
			return fmt.Errorf("no feasible resource for activity %q", activity.ID)
		}
		commitHEFTPlacement(&state, activity, index, *selected)
		if progress != nil {
			_ = progress.Report(ctx, float64(index+1)/float64(len(order)), activity.ID)
		}
	}
	return sink.Emit(ctx, planFromState("heft-candidate-1", "heft", "time", request, state))
}

func heftReadyAt(
	topology domain.NetworkTopology,
	state scheduledState,
	resourceID string,
	predecessorIDs []string,
	dependencyBytes map[string]int64,
) (float64, float64) {
	readyAt, totalTransfer := 0.0, 0.0
	for _, predecessorID := range predecessorIDs {
		predecessor := state.byActivity[predecessorID]
		transfer := transferSeconds(
			topology,
			predecessor.ResourceID,
			resourceID,
			dependencyBytes[predecessorID],
		)
		totalTransfer += transfer
		readyAt = math.Max(readyAt, predecessor.PredictedFinishAt+transfer)
	}
	return readyAt, totalTransfer
}

func evaluateHEFTPlacement(
	state scheduledState,
	resource domain.Resource,
	core string,
	readyAt float64,
	transfer float64,
	runtime float64,
) heftPlacement {
	start := math.Max(readyAt, state.coreFree[core])
	if state.coreOrder[core] == 0 {
		start += resource.BootOverheadSeconds
	}
	start += resource.ContainerOverhead
	finish := start + runtime
	return heftPlacement{
		resource: resource,
		core:     core,
		readyAt:  readyAt,
		transfer: transfer,
		runtime:  runtime,
		start:    start,
		finish:   finish,
		makespan: math.Max(state.makespan, finish),
		cost:     state.cost + runtime*resource.PricePerSecond,
	}
}

func betterHEFTPlacement(candidate heftPlacement, selected *heftPlacement) bool {
	return selected == nil || candidate.makespan < selected.makespan ||
		(candidate.makespan == selected.makespan && candidate.cost < selected.cost)
}

func commitHEFTPlacement(
	state *scheduledState,
	activity domain.Activity,
	sequence int,
	placement heftPlacement,
) {
	assignment := domain.PlanAssignment{
		ID: fmt.Sprintf(
			"assignment-%d-%s-%s",
			sequence,
			activity.ID,
			placement.resource.ID,
		),
		ActivityID:               activity.ID,
		ResourceID:               placement.resource.ID,
		CoreID:                   placement.core,
		OrderOnResource:          state.coreOrder[placement.core],
		Priority:                 activity.Priority,
		PredictedReadyAt:         placement.readyAt,
		PredictedStartAt:         placement.start,
		PredictedFinishAt:        placement.finish,
		PredictedRuntimeSeconds:  placement.runtime,
		PredictedTransferSeconds: placement.transfer,
		PredictedCost:            placement.runtime * placement.resource.PricePerSecond,
		Metadata: map[string]any{
			"scheduleBasis":           "algorithm",
			"expectedDurationSeconds": placement.runtime,
		},
	}
	state.assignments = append(state.assignments, assignment)
	state.byActivity[activity.ID] = assignment
	state.coreFree[placement.core] = placement.finish
	state.coreOrder[placement.core]++
	state.makespan = placement.makespan
	state.cost = placement.cost
}

func heftOrder(request domain.PlanningRequest) ([]domain.Activity, error) {
	topological, err := topologicalOrder(request.Workflow)
	if err != nil {
		return nil, err
	}
	resources := schedulableResources(request)
	byID := map[string]domain.Activity{}
	successors := map[string][]string{}
	for _, activity := range topological {
		byID[activity.ID] = activity
	}
	for _, dependency := range request.Workflow.Dependencies {
		successors[dependency.DependsOnActivityID] = append(successors[dependency.DependsOnActivityID], dependency.ActivityID)
	}
	ranks := map[string]float64{}
	var rank func(string) float64
	rank = func(id string) float64 {
		if value, ok := ranks[id]; ok {
			return value
		}
		average := 0.0
		for _, resource := range resources {
			average += duration(byID[id], resource, request.ActivityProfiles)
		}
		if len(resources) > 0 {
			average /= float64(len(resources))
		}
		maxSuccessor := 0.0
		for _, successor := range successors[id] {
			if value := rank(successor); value > maxSuccessor {
				maxSuccessor = value
			}
		}
		ranks[id] = average + maxSuccessor
		return ranks[id]
	}
	for id := range byID {
		rank(id)
	}
	sort.SliceStable(topological, func(i, j int) bool { return ranks[topological[i].ID] > ranks[topological[j].ID] })
	return topological, nil
}
