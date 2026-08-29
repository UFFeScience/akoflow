package algorithms

import (
	"context"
	"fmt"
	"sort"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type PRISM struct{ Objective string }

func NewPRISMTime() PRISM { return PRISM{Objective: "time"} }
func NewPRISMCost() PRISM { return PRISM{Objective: "cost"} }

func (p PRISM) Descriptor() ports.SchedulerDescriptor {
	id, name, description := "prism-time", "PRISM Time", "PRISM options ranked exclusively by predicted makespan."
	if p.Objective == "cost" {
		id, name, description = "prism-cost", "PRISM Cost", "PRISM options ranked exclusively by predicted cost."
	}
	return ports.SchedulerDescriptor{ID: id, Name: name, Objective: p.Objective, Multiple: true, Description: description, Defaults: map[string]any{"beamWidth": 120, "optionCount": 25}}
}

func (p PRISM) Schedule(ctx context.Context, request domain.PlanningRequest, configuration map[string]any, progress ports.ProgressReporter, sink ports.CandidateSink) error {
	order, err := topologicalOrder(request.Workflow)
	if err != nil {
		return err
	}
	resources := schedulableResources(request)
	if len(resources) == 0 {
		return fmt.Errorf("execution scope has no schedulable resources")
	}
	beamWidth := intOption(configuration, "beamWidth", 120, 1, 10000)
	optionCount := intOption(configuration, "optionCount", 25, 1, 1000)
	states := []scheduledState{initialState()}
	less := func(left, right scheduledState) bool {
		if p.Objective == "cost" {
			if left.cost != right.cost {
				return left.cost < right.cost
			}
			return left.makespan < right.makespan
		}
		if left.makespan != right.makespan {
			return left.makespan < right.makespan
		}
		return left.cost < right.cost
	}
	for index, activity := range order {
		if err := ctx.Err(); err != nil {
			return err
		}
		next := []scheduledState{}
		for _, state := range states {
			for _, resource := range resources {
				if !resourceFeasible(activity, resource) {
					continue
				}
				for _, core := range cores(resource) {
					next = append(next, place(request, state, activity, resource, core, index))
				}
			}
		}
		if len(next) == 0 {
			return fmt.Errorf("no feasible resource for activity %q", activity.ID)
		}
		sort.SliceStable(next, func(i, j int) bool { return less(next[i], next[j]) })
		if len(next) > beamWidth {
			next = next[:beamWidth]
		}
		states = next
		if progress != nil {
			_ = progress.Report(ctx, float64(index+1)/float64(len(order)), activity.ID)
		}
	}
	if len(states) > optionCount {
		states = states[:optionCount]
	}
	for index, state := range states {
		id := fmt.Sprintf("%s-candidate-%d", p.Descriptor().ID, index+1)
		if err := sink.Emit(ctx, planFromState(id, p.Descriptor().ID, p.Objective, request, state)); err != nil {
			return err
		}
	}
	return nil
}
