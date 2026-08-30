package algorithms

import (
	"fmt"
	"math"
	"sort"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type scheduledState struct {
	assignments  []domain.PlanAssignment
	byActivity   map[string]domain.PlanAssignment
	coreFree     map[string]float64
	coreOrder    map[string]int
	networkFlows []scheduledNetworkFlow
	makespan     float64
	cost         float64
	signature    uint64
}

type scheduledNetworkFlow struct {
	sourceResourceID      string
	destinationResourceID string
	readyAt               float64
	deliveredAt           float64
}

func initialState() scheduledState {
	return scheduledState{
		byActivity: map[string]domain.PlanAssignment{},
		coreFree:   map[string]float64{},
		coreOrder:  map[string]int{},
		signature:  14695981039346656037,
	}
}

func cloneState(value scheduledState) scheduledState {
	out := scheduledState{
		assignments:  append([]domain.PlanAssignment{}, value.assignments...),
		byActivity:   map[string]domain.PlanAssignment{},
		coreFree:     map[string]float64{},
		coreOrder:    map[string]int{},
		networkFlows: append([]scheduledNetworkFlow{}, value.networkFlows...),
		makespan:     value.makespan,
		cost:         value.cost,
		signature:    value.signature,
	}
	for key, item := range value.byActivity {
		out.byActivity[key] = item
	}
	for key, item := range value.coreFree {
		out.coreFree[key] = item
	}
	for key, item := range value.coreOrder {
		out.coreOrder[key] = item
	}
	return out
}

func schedulableResources(request domain.PlanningRequest) []domain.Resource {
	allowed := map[string]bool{}
	for _, id := range request.ExecutionScope.EnvironmentVersionIDs {
		allowed[id] = true
	}
	resources := []domain.Resource{}
	for _, resource := range request.Resources {
		if resource.Schedulable && allowed[resource.EnvironmentVersionID] {
			resources = append(resources, resource)
		}
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].ID < resources[j].ID })
	return resources
}

func topologicalOrder(workflow domain.WorkflowVersion) ([]domain.Activity, error) {
	activities := map[string]domain.Activity{}
	indegree := map[string]int{}
	successors := map[string][]string{}
	for _, activity := range workflow.Activities {
		activities[activity.ID] = activity
		indegree[activity.ID] = 0
	}
	for _, dependency := range workflow.Dependencies {
		if _, ok := activities[dependency.ActivityID]; !ok {
			return nil, fmt.Errorf("unknown activity %q", dependency.ActivityID)
		}
		if _, ok := activities[dependency.DependsOnActivityID]; !ok {
			return nil, fmt.Errorf("unknown predecessor %q", dependency.DependsOnActivityID)
		}
		indegree[dependency.ActivityID]++
		successors[dependency.DependsOnActivityID] = append(successors[dependency.DependsOnActivityID], dependency.ActivityID)
	}
	ready := []string{}
	for id, degree := range indegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	ordered := []domain.Activity{}
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, activities[id])
		for _, successor := range successors[id] {
			indegree[successor]--
			if indegree[successor] == 0 {
				ready = append(ready, successor)
				sort.Strings(ready)
			}
		}
	}
	if len(ordered) != len(workflow.Activities) {
		return nil, fmt.Errorf("workflow contains a cycle")
	}
	return ordered, nil
}

func predecessors(workflow domain.WorkflowVersion) map[string][]string {
	items := map[string][]string{}
	for _, dependency := range workflow.Dependencies {
		items[dependency.ActivityID] = append(items[dependency.ActivityID], dependency.DependsOnActivityID)
	}
	return items
}

func dataBytes(workflow domain.WorkflowVersion) map[string]map[string]int64 {
	items := map[string]map[string]int64{}
	for _, dependency := range workflow.DataDependencies {
		if items[dependency.ConsumerActivityID] == nil {
			items[dependency.ConsumerActivityID] = map[string]int64{}
		}
		items[dependency.ConsumerActivityID][dependency.ProducerActivityID] += dependency.SizeBytes
	}
	return items
}

func duration(activity domain.Activity, resource domain.Resource, profiles []domain.ActivityResourceProfile) float64 {
	for _, profile := range profiles {
		if profile.ActivityTypeID == activity.ActivityTypeID && profile.ResourceID == resource.ID && profile.RuntimeSeconds > 0 {
			return profile.RuntimeSeconds
		}
	}
	base := 1.0
	if activity.Simulation != nil && activity.Simulation.DurationSeconds > 0 {
		base = activity.Simulation.DurationSeconds
	}
	speedup := resource.ComputeSpeedup
	if speedup <= 0 {
		speedup = 1
	}
	return base / speedup
}

func resourceFeasible(activity domain.Activity, resource domain.Resource) bool {
	if opaqueBatchTarget(resource) {
		return true
	}
	return resource.CPUCapacity >= activity.Resources.CPU && resource.MemoryBytes >= activity.Resources.MemoryBytes
}

// An abstract queue, partition, or reservation does not expose stable cores to
// the planner. A discovered HPC machine does, even when its jobs are submitted
// through a batch scheduler.
func opaqueBatchTarget(resource domain.Resource) bool {
	if resource.ExecutionTarget != domain.ExecutionTargetBatch {
		return false
	}
	return resource.Type == domain.ResourceBatchQueue ||
		resource.Type == domain.ResourceHPCPartition ||
		resource.Type == domain.ResourceSlurmReservation
}

func cores(resource domain.Resource) []string {
	count := resource.CPUCores
	if count < 1 {
		count = 1
	}
	items := make([]string, count)
	for index := range items {
		items[index] = fmt.Sprintf("%s-core-%d", resource.ID, index+1)
	}
	if opaqueBatchTarget(resource) {
		return []string{resource.ID + "-slot"}
	}
	return items
}

func transferSeconds(topology domain.NetworkTopology, source, target string, bytes int64) float64 {
	if source == target || bytes <= 0 {
		return 0
	}
	for _, link := range topology.Links {
		if (link.SourceResourceID == source && link.TargetResourceID == target) || (link.Bidirectional && link.SourceResourceID == target && link.TargetResourceID == source) {
			return link.TransferSeconds(bytes)
		}
	}
	return 0
}

func prismTransferSeconds(
	topology domain.NetworkTopology,
	source string,
	target string,
	bytes int64,
	readyAt float64,
	knownFlows []scheduledNetworkFlow,
) float64 {
	if source == target || bytes <= 0 {
		return 0
	}

	for _, link := range topology.Links {
		matchesForward := link.SourceResourceID == source && link.TargetResourceID == target
		matchesReverse := link.Bidirectional && link.SourceResourceID == target && link.TargetResourceID == source
		if !matchesForward && !matchesReverse {
			continue
		}

		concurrency := 1
		for _, flow := range knownFlows {
			activeAtReady := flow.readyAt <= readyAt && readyAt < flow.deliveredAt
			sharesEndpoint := flow.sourceResourceID == source || flow.destinationResourceID == target
			if activeAtReady && sharesEndpoint {
				concurrency++
			}
		}

		if link.BandwidthBitsPerSecond <= 0 {
			return link.TransferSeconds(bytes)
		}
		dataSeconds := float64(bytes) / (link.BandwidthBitsPerSecond / 8.0)
		return link.LatencySeconds + dataSeconds*float64(concurrency)
	}

	return 0
}

func place(request domain.PlanningRequest, state scheduledState, activity domain.Activity, resource domain.Resource, core string, sequence int) scheduledState {
	out := cloneState(state)
	readyAt, transfers := 0.0, 0.0
	preds := predecessors(request.Workflow)[activity.ID]
	bytes := dataBytes(request.Workflow)[activity.ID]
	for _, predecessorID := range preds {
		predecessor := state.byActivity[predecessorID]
		transfer := transferSeconds(request.NetworkTopology, predecessor.ResourceID, resource.ID, bytes[predecessorID])
		transfers += transfer
		if predecessor.PredictedFinishAt+transfer > readyAt {
			readyAt = predecessor.PredictedFinishAt + transfer
		}
	}
	start := math.Max(readyAt, state.coreFree[core])
	runtime := duration(activity, resource, request.ActivityProfiles)
	if state.coreOrder[core] == 0 {
		start += resource.BootOverheadSeconds
	}
	start += resource.ContainerOverhead
	finish := start + runtime
	assignment := domain.PlanAssignment{
		ID: fmt.Sprintf("assignment-%d-%s-%s", sequence, activity.ID, resource.ID), ActivityID: activity.ID,
		ResourceID: resource.ID, CoreID: core, OrderOnResource: state.coreOrder[core], Priority: activity.Priority,
		PredictedReadyAt: readyAt, PredictedStartAt: start, PredictedFinishAt: finish,
		PredictedRuntimeSeconds: runtime, PredictedTransferSeconds: transfers,
		PredictedCost: runtime * resource.PricePerSecond,
		Metadata:      map[string]any{"scheduleBasis": "algorithm", "expectedDurationSeconds": runtime},
	}
	out.assignments = append(out.assignments, assignment)
	out.byActivity[activity.ID] = assignment
	out.coreFree[core] = finish
	out.coreOrder[core]++
	out.cost += assignment.PredictedCost
	if finish > out.makespan {
		out.makespan = finish
	}
	return out
}

func placePRISM(
	request domain.PlanningRequest,
	state scheduledState,
	activity domain.Activity,
	resource domain.Resource,
	core string,
	sequence int,
) scheduledState {
	out := cloneState(state)
	readyAt, transfers := 0.0, 0.0
	preds := predecessors(request.Workflow)[activity.ID]
	bytes := dataBytes(request.Workflow)[activity.ID]
	for _, predecessorID := range preds {
		predecessor := state.byActivity[predecessorID]
		flowReadyAt := predecessor.PredictedFinishAt
		transfer := prismTransferSeconds(
			request.NetworkTopology,
			predecessor.ResourceID,
			resource.ID,
			bytes[predecessorID],
			flowReadyAt,
			out.networkFlows,
		)
		transfers += transfer
		deliveredAt := flowReadyAt + transfer
		if deliveredAt > readyAt {
			readyAt = deliveredAt
		}
		if predecessor.ResourceID != resource.ID && bytes[predecessorID] > 0 {
			out.networkFlows = append(out.networkFlows, scheduledNetworkFlow{
				sourceResourceID:      predecessor.ResourceID,
				destinationResourceID: resource.ID,
				readyAt:               flowReadyAt,
				deliveredAt:           deliveredAt,
			})
		}
	}

	start := math.Max(readyAt, state.coreFree[core])
	runtime := duration(activity, resource, request.ActivityProfiles)
	if state.coreOrder[core] == 0 {
		start += resource.BootOverheadSeconds
	}
	start += resource.ContainerOverhead
	finish := start + runtime
	assignment := domain.PlanAssignment{
		ID:                       fmt.Sprintf("assignment-%d-%s-%s", sequence, activity.ID, resource.ID),
		ActivityID:               activity.ID,
		ResourceID:               resource.ID,
		CoreID:                   core,
		OrderOnResource:          state.coreOrder[core],
		Priority:                 activity.Priority,
		PredictedReadyAt:         readyAt,
		PredictedStartAt:         start,
		PredictedFinishAt:        finish,
		PredictedRuntimeSeconds:  runtime,
		PredictedTransferSeconds: transfers,
		PredictedCost:            runtime * resource.PricePerSecond,
		Metadata: map[string]any{
			"scheduleBasis":           "algorithm",
			"expectedDurationSeconds": runtime,
			"networkContentionModel":  "known-active-flows",
		},
	}
	out.assignments = append(out.assignments, assignment)
	out.byActivity[activity.ID] = assignment
	out.coreFree[core] = finish
	out.coreOrder[core]++
	out.cost += assignment.PredictedCost
	out.signature = extendScheduleSignature(
		state.signature,
		activity.ID,
		resource.ID,
		core,
	)
	if finish > out.makespan {
		out.makespan = finish
	}
	return out
}

func extendScheduleSignature(seed uint64, values ...string) uint64 {
	const prime = uint64(1099511628211)
	value := seed
	for _, item := range values {
		for index := 0; index < len(item); index++ {
			value ^= uint64(item[index])
			value *= prime
		}
		value ^= 0xff
		value *= prime
	}
	return value
}

func planFromState(id, algorithm, objective string, request domain.PlanningRequest, state scheduledState) domain.SchedulePlan {
	assignments := append([]domain.PlanAssignment{}, state.assignments...)
	for index := range assignments {
		assignments[index].ID = fmt.Sprintf("%s-%s", id, assignments[index].ActivityID)
		assignments[index].PlanID = id
	}
	feasible := (request.DeadlineSeconds <= 0 || state.makespan <= request.DeadlineSeconds) && (request.Budget <= 0 || state.cost <= request.Budget)
	return domain.SchedulePlan{ID: id, WorkflowVersionID: request.Workflow.ID, ExecutionScopeID: request.ExecutionScope.ID,
		NetworkTopologyID: request.NetworkTopology.ID, Source: domain.PlanningSourcePlugin, Algorithm: algorithm,
		AlgorithmVersion: "1", Objective: objective, DeadlineSeconds: request.DeadlineSeconds, Budget: request.Budget,
		Predicted: domain.PredictedMetrics{MakespanSeconds: state.makespan, Cost: state.cost, Feasible: feasible}, Assignments: assignments}
}

func intOption(configuration map[string]any, key string, fallback, minimum, maximum int) int {
	value := fallback
	if raw, ok := configuration[key].(float64); ok {
		value = int(raw)
	}
	if raw, ok := configuration[key].(int); ok {
		value = raw
	}
	if value < minimum {
		value = minimum
	}
	if value > maximum {
		value = maximum
	}
	return value
}
