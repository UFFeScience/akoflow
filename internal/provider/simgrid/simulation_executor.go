package simgrid

import (
	"context"
	"fmt"
	"sort"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/infrastructure/plugins/planning"
)

type SimulationExecutor struct {
	validator planning.Validator
}

func NewSimulationExecutor() *SimulationExecutor {
	return &SimulationExecutor{validator: planning.NewValidator()}
}

func (e *SimulationExecutor) Execute(ctx context.Context, request Request) (domain.ExecutionTrace, error) {
	if request.Run.Mode != domain.ExecutionModeSimulation {
		return domain.ExecutionTrace{}, fmt.Errorf("simulation executor cannot execute mode %q", request.Run.Mode)
	}
	if err := e.validator.Validate(request.Plan, request.Workflow, request.Resources, request.ExecutionScope, request.NetworkTopology); err != nil {
		return domain.ExecutionTrace{}, err
	}
	select {
	case <-ctx.Done():
		return domain.ExecutionTrace{}, ctx.Err()
	default:
	}

	model := indexRequest(request)
	completed, transfers, err := simulate(request, model)
	if err != nil {
		return domain.ExecutionTrace{}, err
	}
	return buildTrace(request, completed, transfers), nil
}

type indexedRequest struct {
	resources    map[string]domain.Resource
	assignments  map[string]domain.PlanAssignment
	profiles     map[string]domain.ActivityResourceProfile
	predecessors map[string][]string
	dataBytes    map[string]int64
	lanePrevious map[string]string
}

func indexRequest(request Request) indexedRequest {
	model := indexedRequest{
		resources:    make(map[string]domain.Resource, len(request.Resources)),
		assignments:  make(map[string]domain.PlanAssignment, len(request.Plan.Assignments)),
		profiles:     make(map[string]domain.ActivityResourceProfile, len(request.ActivityProfiles)),
		predecessors: make(map[string][]string), dataBytes: make(map[string]int64),
		lanePrevious: make(map[string]string),
	}
	for _, resource := range request.Resources {
		model.resources[resource.ID] = resource
	}
	for _, assignment := range request.Plan.Assignments {
		model.assignments[assignment.ActivityID] = assignment
	}
	for _, profile := range request.ActivityProfiles {
		model.profiles[profile.ActivityTypeID+"\x00"+profile.ResourceID] = profile
	}
	for _, dependency := range request.Workflow.Dependencies {
		model.predecessors[dependency.ActivityID] = append(
			model.predecessors[dependency.ActivityID], dependency.DependsOnActivityID,
		)
	}
	for _, dependency := range request.Workflow.DataDependencies {
		key := dependency.ProducerActivityID + "\x00" + dependency.ConsumerActivityID
		model.dataBytes[key] += dependency.SizeBytes
	}
	byLane := make(map[string][]domain.PlanAssignment)
	for _, assignment := range request.Plan.Assignments {
		lane := assignment.ResourceID + "\x00" + assignment.CoreID + "\x00" + assignment.SlotID
		byLane[lane] = append(byLane[lane], assignment)
	}
	for _, lane := range byLane {
		sort.Slice(lane, func(i, j int) bool { return lane[i].OrderOnResource < lane[j].OrderOnResource })
		for i := 1; i < len(lane); i++ {
			model.lanePrevious[lane[i].ActivityID] = lane[i-1].ActivityID
		}
	}
	return model
}

func simulate(
	request Request,
	model indexedRequest,
) (map[string]domain.TaskExecution, []domain.DataTransfer, error) {
	completed := make(map[string]domain.TaskExecution, len(request.Workflow.Activities))
	transfers := make([]domain.DataTransfer, 0)
	for len(completed) < len(request.Workflow.Activities) {
		progress := false
		for _, activity := range request.Workflow.Activities {
			if _, done := completed[activity.ID]; done {
				continue
			}
			if !allCompleted(model.predecessors[activity.ID], completed) {
				continue
			}
			previous := model.lanePrevious[activity.ID]
			if previous != "" {
				if _, done := completed[previous]; !done {
					continue
				}
			}

			assignment := model.assignments[activity.ID]
			resource := model.resources[assignment.ResourceID]
			dataReadyAt, transferTotal, activityTransfers, err := resolveTransfers(
				request, activity, resource, model, completed,
			)
			if err != nil {
				return nil, nil, err
			}
			transfers = append(transfers, activityTransfers...)
			resourceReadyAt := 0.0
			if previous != "" {
				resourceReadyAt = completed[previous].FinishedAt
			}
			start := max(dataReadyAt, resourceReadyAt)
			runtimeSeconds := resolveRuntime(activity, resource, model.profiles)
			overhead := resolveAssignmentOverhead(assignment, resource)
			finish := start + runtimeSeconds + overhead
			execution := domain.TaskExecution{
				ID: fmt.Sprintf("%s:%s", request.Run.ID, activity.ID), ExecutionRunID: request.Run.ID,
				PlanAssignmentID: assignment.ID, ActivityID: activity.ID,
				PlannedResourceID: resource.ID, AllocatedResourceID: resource.ID, Attempt: 1,
				Status: domain.TaskCompleted, ReadyAt: dataReadyAt, DataReadyAt: dataReadyAt,
				QueuedAt: dataReadyAt, StartedAt: start, FinishedAt: finish,
				RuntimeSeconds: runtimeSeconds, QueueSeconds: start - dataReadyAt,
				TransferSeconds: transferTotal, OverheadSeconds: overhead,
				Cost: (runtimeSeconds + overhead) * resource.PricePerSecond,
			}
			completed[activity.ID] = execution
			progress = true
		}
		if !progress {
			return nil, nil, fmt.Errorf("plan cannot progress: dependency and resource orders form a cycle")
		}
	}
	return completed, transfers, nil
}

func resolveTransfers(
	request Request,
	activity domain.Activity,
	resource domain.Resource,
	model indexedRequest,
	completed map[string]domain.TaskExecution,
) (float64, float64, []domain.DataTransfer, error) {
	dataReadyAt, transferTotal := 0.0, 0.0
	transfers := make([]domain.DataTransfer, 0)
	for _, predecessorID := range model.predecessors[activity.ID] {
		predecessor := completed[predecessorID]
		readyAt := predecessor.FinishedAt
		bytes := model.dataBytes[predecessorID+"\x00"+activity.ID]
		if predecessor.AllocatedResourceID != resource.ID && bytes > 0 {
			duration, transferCost, ok := resolveNetworkPath(
				request.NetworkTopology.Links, predecessor.AllocatedResourceID, resource.ID, bytes,
			)
			if !ok {
				return 0, 0, nil, fmt.Errorf(
					"no network link from %q to %q", predecessor.AllocatedResourceID, resource.ID,
				)
			}
			transfer := domain.DataTransfer{
				ID:             fmt.Sprintf("%s:%s:%s", request.Run.ID, predecessorID, activity.ID),
				ExecutionRunID: request.Run.ID, ProducerActivityID: predecessorID,
				ConsumerActivityID: activity.ID, SourceResourceID: predecessor.AllocatedResourceID,
				TargetResourceID: resource.ID, Bytes: bytes, StartedAt: predecessor.FinishedAt,
				FinishedAt: predecessor.FinishedAt + duration, DurationSeconds: duration,
				Cost: transferCost,
			}
			transfers = append(transfers, transfer)
			readyAt, transferTotal = transfer.FinishedAt, transferTotal+duration
		}
		dataReadyAt = max(dataReadyAt, readyAt)
	}
	return dataReadyAt, transferTotal, transfers, nil
}

func buildTrace(
	request Request,
	completed map[string]domain.TaskExecution,
	transfers []domain.DataTransfer,
) domain.ExecutionTrace {
	tasks := make([]domain.TaskExecution, 0, len(completed))
	metrics := domain.ExecutionMetrics{Feasible: true}
	for _, activity := range request.Workflow.Activities {
		task := completed[activity.ID]
		tasks = append(tasks, task)
	}
	applyResourceActiveCosts(tasks, request.Resources)
	for _, task := range tasks {
		metrics.MakespanSeconds = max(metrics.MakespanSeconds, task.FinishedAt)
		metrics.ComputeSeconds += task.RuntimeSeconds
		metrics.QueueSeconds += task.QueueSeconds
		metrics.TransferSeconds += task.TransferSeconds
		metrics.OverheadSeconds += task.OverheadSeconds
		metrics.Cost += task.Cost
	}
	for _, transfer := range transfers {
		metrics.Cost += transfer.Cost
	}
	if request.Plan.DeadlineSeconds > 0 && metrics.MakespanSeconds > request.Plan.DeadlineSeconds {
		metrics.Feasible = false
	}
	if request.Plan.Budget > 0 && metrics.Cost > request.Plan.Budget {
		metrics.Feasible = false
	}
	return domain.ExecutionTrace{
		RunID: request.Run.ID, PlanID: request.Plan.ID, Mode: request.Run.Mode,
		Predicted: request.Plan.Predicted, Executed: metrics, Tasks: tasks, Transfers: transfers,
	}
}

func resolveAssignmentOverhead(assignment domain.PlanAssignment, resource domain.Resource) float64 {
	boot, hasBoot := numberMetadata(assignment.Metadata, "bootOverheadSeconds")
	container, hasContainer := numberMetadata(assignment.Metadata, "containerOverheadSeconds")
	if !hasBoot {
		boot = resource.BootOverheadSeconds
	}
	if !hasContainer {
		container = resource.ContainerOverhead
	}
	return boot + container
}

// applyResourceActiveCosts models VM/node billing once per active resource
// window instead of charging the full machine price independently to every
// colocated activity. The total is distributed among tasks for traceability.
func applyResourceActiveCosts(tasks []domain.TaskExecution, resources []domain.Resource) {
	prices := make(map[string]float64, len(resources))
	for _, resource := range resources {
		prices[resource.ID] = resource.PricePerSecond
	}
	byResource := make(map[string][]int)
	for index := range tasks {
		byResource[tasks[index].AllocatedResourceID] = append(byResource[tasks[index].AllocatedResourceID], index)
		tasks[index].Cost = 0
	}
	for resourceID, indexes := range byResource {
		price := prices[resourceID]
		if price <= 0 || len(indexes) == 0 {
			continue
		}
		start, finish, runtimeTotal := tasks[indexes[0]].StartedAt, tasks[indexes[0]].FinishedAt, 0.0
		for _, index := range indexes {
			start = min(start, tasks[index].StartedAt)
			finish = max(finish, tasks[index].FinishedAt)
			runtimeTotal += tasks[index].RuntimeSeconds
		}
		total := max(0, finish-start) * price
		for _, index := range indexes {
			share := 1 / float64(len(indexes))
			if runtimeTotal > 0 {
				share = tasks[index].RuntimeSeconds / runtimeTotal
			}
			tasks[index].Cost = total * share
		}
	}
}

func allCompleted(ids []string, completed map[string]domain.TaskExecution) bool {
	for _, id := range ids {
		if _, ok := completed[id]; !ok {
			return false
		}
	}
	return true
}

func resolveRuntime(activity domain.Activity, resource domain.Resource, profiles map[string]domain.ActivityResourceProfile) float64 {
	if base, ok := numberMetadata(activity.Metadata, "baseRuntimeSeconds"); ok && resource.ComputeSpeedup > 0 {
		return base / resource.ComputeSpeedup
	}
	if profile, ok := profiles[activity.ActivityTypeID+"\x00"+resource.ID]; ok {
		return profile.RuntimeSeconds
	}
	if activity.Simulation != nil {
		return activity.Simulation.DurationSeconds
	}
	return 0
}

func numberMetadata(metadata map[string]any, key string) (float64, bool) {
	value, ok := metadata[key]
	if !ok {
		return 0, false
	}
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	default:
		return 0, false
	}
}

func resolveNetworkPath(links []domain.NetworkLink, source, target string, bytes int64) (float64, float64, bool) {
	type edge struct {
		to      string
		seconds float64
		cost    float64
	}
	graph := make(map[string][]edge)
	for _, link := range links {
		value := edge{to: link.TargetResourceID, seconds: link.TransferSeconds(bytes), cost: float64(bytes) * link.PricePerByte}
		graph[link.SourceResourceID] = append(graph[link.SourceResourceID], value)
		if link.Bidirectional {
			value.to = link.SourceResourceID
			graph[link.TargetResourceID] = append(graph[link.TargetResourceID], value)
		}
	}
	distance := map[string]float64{source: 0}
	cost := map[string]float64{source: 0}
	visited := make(map[string]bool)
	for {
		current := ""
		best := 0.0
		for node, value := range distance {
			if !visited[node] && (current == "" || value < best) {
				current, best = node, value
			}
		}
		if current == "" {
			return 0, 0, false
		}
		if current == target {
			return distance[current], cost[current], true
		}
		visited[current] = true
		for _, candidate := range graph[current] {
			newDistance := distance[current] + candidate.seconds
			oldDistance, exists := distance[candidate.to]
			if !exists || newDistance < oldDistance {
				distance[candidate.to] = newDistance
				cost[candidate.to] = cost[current] + candidate.cost
			}
		}
	}
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
