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

type heftInterval struct {
	start, finish   float64
	assignmentIndex int
}

func (HEFT) Descriptor() ports.SchedulerDescriptor {
	return ports.SchedulerDescriptor{ID: "heft", Origin: "native", Name: "HEFT", Objective: "time", Multiple: false, Description: "Classic heterogeneous earliest-finish-time baseline."}
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
	candidates := heftCandidates(resources)
	preds := predecessors(request.Workflow)
	bytes := dataBytes(request.Workflow)
	state := initialState()
	calendars := map[string][]heftInterval{}
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
				placement := evaluateHEFTCalendar(
					state,
					resource,
					core,
					readyAt,
					transfer,
					runtime,
					calendars[core],
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
		calendar := calendars[selected.core]
		interval := heftInterval{selected.start - selected.resource.ContainerOverhead, selected.finish, len(state.assignments) - 1}
		position := sort.Search(len(calendar), func(i int) bool { return calendar[i].start > interval.start })
		calendar = append(calendar, heftInterval{})
		copy(calendar[position+1:], calendar[position:])
		calendar[position] = interval
		calendars[selected.core] = calendar
		if progress != nil {
			_ = progress.Report(ctx, float64(index+1)/float64(len(order)), activity.ID)
		}
	}
	// Insertion can place a later-ranked task before an already placed task.
	// Persist chronological lane order rather than rank/selection order.
	for _, calendar := range calendars {
		for order, interval := range calendar {
			state.assignments[interval.assignmentIndex].OrderOnResource = order
		}
	}
	plan := planFromState("heft-candidate-1", "heft", "time", request, state)
	plan.AlgorithmVersion = "canonical-insertion-v2"
	if plan.Metadata == nil {
		plan.Metadata = map[string]any{}
	}
	plan.Metadata["reference"] = "10.1109/71.993206"
	plan.Metadata["placement"] = "minimum-activity-EFT-with-gap-insertion"
	plan.Metadata["rank"] = "average-feasible-node-runtime-plus-average-directed-communication"
	plan.Metadata["coreExtension"] = "independent lanes within each physical resource; same-resource communication is zero"
	return sink.Emit(ctx, plan)
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
	calendar := []heftInterval{}
	for i, a := range state.assignments {
		if a.CoreID == core {
			calendar = append(calendar, heftInterval{a.PredictedStartAt - resource.ContainerOverhead, a.PredictedFinishAt, i})
		}
	}
	sort.SliceStable(calendar, func(i, j int) bool { return calendar[i].start < calendar[j].start })
	return evaluateHEFTCalendar(state, resource, core, readyAt, transfer, runtime, calendar)
}

func evaluateHEFTCalendar(state scheduledState, resource domain.Resource, core string, readyAt, transfer, runtime float64, calendar []heftInterval) heftPlacement {
	// Resource boot is an absolute initial availability, not a per-task delay.
	start := math.Max(readyAt, resource.BootOverheadSeconds)
	service := runtime + resource.ContainerOverhead
	for _, busy := range calendar {
		occupiedStart := busy.start
		if start+service <= occupiedStart+1e-12 {
			break
		}
		if start < busy.finish {
			start = busy.finish
		}
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
	return selected == nil || candidate.finish < selected.finish ||
		(candidate.finish == selected.finish && candidate.cost < selected.cost)
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
	if placement.resource.BootOverheadSeconds > 0 || placement.resource.ContainerOverhead > 0 {
		assignment.Metadata["bootOverheadSeconds"] = 0.0
		assignment.Metadata["resourceAvailableAtSeconds"] = placement.resource.BootOverheadSeconds
		assignment.Metadata["containerOverheadSeconds"] = placement.resource.ContainerOverhead
	}
	state.assignments = append(state.assignments, assignment)
	state.byActivity[activity.ID] = assignment
	state.coreFree[placement.core] = math.Max(state.coreFree[placement.core], placement.finish)
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
	edgeBytes := dataBytes(request.Workflow)
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
		feasible := 0
		for _, resource := range resources {
			if resourceFeasible(byID[id], resource) {
				average += duration(byID[id], resource, request.ActivityProfiles)
				feasible++
			}
		}
		if feasible > 0 {
			average /= float64(feasible)
		}
		maxSuccessor := 0.0
		for _, successor := range successors[id] {
			communication, pairs := 0.0, 0
			for _, from := range resources {
				for _, to := range resources {
					if from.ID != to.ID && resourceFeasible(byID[id], from) && resourceFeasible(byID[successor], to) {
						communication += transferSeconds(request.NetworkTopology, from.ID, to.ID, edgeBytes[successor][id])
						pairs++
					}
				}
			}
			if pairs > 0 {
				communication /= float64(pairs)
			}
			if value := communication + rank(successor); value > maxSuccessor {
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

func heftCandidates(resources []domain.Resource) []heftResource {
	candidates := make([]heftResource, 0, len(resources))
	for _, resource := range resources {
		candidates = append(candidates, heftResource{
			resource: resource,
			cores:    cores(resource),
		})
	}

	return candidates
}
