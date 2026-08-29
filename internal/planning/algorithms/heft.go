package algorithms

import (
	"context"
	"fmt"
	"sort"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type HEFT struct{}

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
	state := initialState()
	for index, activity := range order {
		if err := ctx.Err(); err != nil {
			return err
		}
		var selected *scheduledState
		for _, resource := range resources {
			if !resourceFeasible(activity, resource) {
				continue
			}
			for _, core := range cores(resource) {
				candidate := place(request, state, activity, resource, core, index)
				if selected == nil || candidate.makespan < selected.makespan || (candidate.makespan == selected.makespan && candidate.cost < selected.cost) {
					copy := candidate
					selected = &copy
				}
			}
		}
		if selected == nil {
			return fmt.Errorf("no feasible resource for activity %q", activity.ID)
		}
		state = *selected
		if progress != nil {
			_ = progress.Report(ctx, float64(index+1)/float64(len(order)), activity.ID)
		}
	}
	return sink.Emit(ctx, planFromState("heft-candidate-1", "heft", "time", request, state))
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
