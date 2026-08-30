package algorithms

import (
	"container/heap"
	"fmt"
	"math"

	"github.com/UFFeScience/akoflow/internal/domain"
)

const prismDetailedEvaluationLimit = 512

type prismEvaluationDependency struct {
	consumer int
	bytes    int64
}

type prismEvaluationTask struct {
	assignment      domain.PlanAssignment
	resource        domain.Resource
	remainingInputs int
	dataReadyAt     float64
	transferSeconds float64
	laneReady       bool
	started         bool
	completed       bool
}

type prismEvaluationTaskEvent struct {
	activity int
	finishAt float64
}

type prismEvaluationTaskQueue []prismEvaluationTaskEvent

func (queue prismEvaluationTaskQueue) Len() int { return len(queue) }
func (queue prismEvaluationTaskQueue) Less(i, j int) bool {
	return queue[i].finishAt < queue[j].finishAt
}
func (queue prismEvaluationTaskQueue) Swap(i, j int) { queue[i], queue[j] = queue[j], queue[i] }
func (queue *prismEvaluationTaskQueue) Push(value any) {
	*queue = append(*queue, value.(prismEvaluationTaskEvent))
}
func (queue *prismEvaluationTaskQueue) Pop() any {
	old := *queue
	value := old[len(old)-1]
	*queue = old[:len(old)-1]
	return value
}

type prismEvaluationFlow struct {
	consumer  int
	bytes     int64
	remaining float64
	startedAt float64
	payloadAt float64
	route     compactPRISMRoute
	active    bool
}

func reevaluateCompleteCompactPRISMStates(
	search compactPRISMContext,
	states []compactPRISMState,
) ([]compactPRISMState, error) {
	if len(search.activities) > prismDetailedEvaluationLimit {
		return states, nil
	}
	result := make([]compactPRISMState, 0, len(states))
	for _, state := range states {
		evaluated, err := evaluateCompleteCompactPRISMState(search, state)
		if err != nil {
			return nil, err
		}
		result = append(result, evaluated)
	}
	return result, nil
}

func evaluateCompleteCompactPRISMState(
	search compactPRISMContext,
	state compactPRISMState,
) (compactPRISMState, error) {
	assignments := compactPRISMAssignments(state)
	if len(assignments) != len(search.activities) {
		return state, nil
	}
	resourceByID := make(map[string]domain.Resource, len(search.resources))
	for _, resource := range search.resources {
		resourceByID[resource.resource.ID] = resource.resource
	}
	tasks := make([]prismEvaluationTask, len(search.activities))
	assignmentOrdinal := make(map[string]int, len(assignments))
	for index, assignment := range assignments {
		ordinal, exists := search.activityOrdinal[assignment.ActivityID]
		if !exists {
			return state, fmt.Errorf("evaluate PRISM candidate: unknown activity %q", assignment.ActivityID)
		}
		resource, exists := resourceByID[assignment.ResourceID]
		if !exists {
			return state, fmt.Errorf("evaluate PRISM candidate: unknown resource %q", assignment.ResourceID)
		}
		tasks[ordinal] = prismEvaluationTask{assignment: assignment, resource: resource}
		assignmentOrdinal[assignment.ActivityID] = ordinal
		_ = index
	}
	outgoing := make([][]prismEvaluationDependency, len(tasks))
	data := dataBytes(search.request.Workflow)
	for _, dependency := range search.request.Workflow.Dependencies {
		producer := assignmentOrdinal[dependency.DependsOnActivityID]
		consumer := assignmentOrdinal[dependency.ActivityID]
		outgoing[producer] = append(outgoing[producer], prismEvaluationDependency{
			consumer: consumer,
			bytes:    data[dependency.ActivityID][dependency.DependsOnActivityID],
		})
		tasks[consumer].remainingInputs++
	}
	laneSuccessor := make([]int, len(tasks))
	for index := range laneSuccessor {
		laneSuccessor[index] = -1
		tasks[index].laneReady = true
	}
	byLane := make(map[string][]int)
	for ordinal, task := range tasks {
		lane := task.assignment.ResourceID + "\x00" + task.assignment.CoreID + "\x00" + task.assignment.SlotID
		byLane[lane] = append(byLane[lane], ordinal)
	}
	for _, lane := range byLane {
		for left := 0; left < len(lane); left++ {
			for right := left + 1; right < len(lane); right++ {
				if tasks[lane[right]].assignment.OrderOnResource < tasks[lane[left]].assignment.OrderOnResource {
					lane[left], lane[right] = lane[right], lane[left]
				}
			}
		}
		for index := 1; index < len(lane); index++ {
			tasks[lane[index]].laneReady = false
			laneSuccessor[lane[index-1]] = lane[index]
		}
	}

	clock := 0.0
	completed := 0
	taskEvents := &prismEvaluationTaskQueue{}
	heap.Init(taskEvents)
	flows := make([]prismEvaluationFlow, 0, len(search.request.Workflow.Dependencies))
	resourceStart := make(map[string]float64)
	resourceFinish := make(map[string]float64)
	resourceUsed := make(map[string]bool)
	transferCost := 0.0

	for completed < len(tasks) {
		started := prismStartReadyTasks(tasks, taskEvents, clock, resourceStart, resourceFinish, resourceUsed)
		if completed == len(tasks) {
			break
		}
		rates, nextFlowAt := prismEvaluationFlowRates(flows, clock)
		nextAt := nextFlowAt
		if taskEvents.Len() > 0 {
			nextAt = math.Min(nextAt, (*taskEvents)[0].finishAt)
		}
		if math.IsInf(nextAt, 1) {
			if started {
				continue
			}
			return state, fmt.Errorf("evaluate PRISM candidate: execution cannot progress")
		}
		delta := math.Max(0, nextAt-clock)
		for index := range flows {
			if flows[index].active && flows[index].payloadAt <= clock {
				flows[index].remaining = math.Max(0, flows[index].remaining-rates[index]*delta)
				// At large simulated timestamps, a sub-nanosecond completion
				// increment can round back to the current clock. Snap that final
				// fraction to zero so the event loop always makes progress.
				if rates[index] > 0 && flows[index].remaining/rates[index] <= 1e-9 {
					flows[index].remaining = 0
				}
			}
		}
		clock = nextAt

		for taskEvents.Len() > 0 && (*taskEvents)[0].finishAt <= clock+1e-9 {
			event := heap.Pop(taskEvents).(prismEvaluationTaskEvent)
			if tasks[event.activity].completed {
				continue
			}
			tasks[event.activity].completed = true
			completed++
			if successor := laneSuccessor[event.activity]; successor >= 0 {
				tasks[successor].laneReady = true
			}
			for _, dependency := range outgoing[event.activity] {
				producer := tasks[event.activity]
				consumer := &tasks[dependency.consumer]
				if dependency.bytes <= 0 || producer.assignment.ResourceID == consumer.assignment.ResourceID {
					prismSatisfyEvaluationInput(consumer, clock, 0)
					continue
				}
				route, exists := compactPRISMShortestRoute(
					search.request.NetworkTopology,
					producer.assignment.ResourceID,
					consumer.assignment.ResourceID,
					dependency.bytes,
				)
				if !exists {
					return state, fmt.Errorf(
						"evaluate PRISM candidate: no route from %q to %q",
						producer.assignment.ResourceID,
						consumer.assignment.ResourceID,
					)
				}
				latency := 0.0
				for _, hop := range route.hops {
					latency += hop.link.LatencySeconds
					transferCost += float64(dependency.bytes) * hop.link.PricePerByte
				}
				flows = append(flows, prismEvaluationFlow{
					consumer: dependency.consumer, bytes: dependency.bytes,
					remaining: float64(dependency.bytes), startedAt: clock,
					payloadAt: clock + latency, route: route, active: true,
				})
			}
		}
		for index := range flows {
			if !flows[index].active || flows[index].payloadAt > clock+1e-9 || flows[index].remaining > 1e-6 {
				continue
			}
			flows[index].active = false
			prismSatisfyEvaluationInput(
				&tasks[flows[index].consumer],
				clock,
				clock-flows[index].startedAt,
			)
		}
	}

	cost := transferCost
	for resourceID, start := range resourceStart {
		cost += math.Max(0, resourceFinish[resourceID]-start) * resourceByID[resourceID].PricePerSecond
	}
	evaluatedAssignments := make([]domain.PlanAssignment, len(assignments))
	for index, assignment := range assignments {
		ordinal := assignmentOrdinal[assignment.ActivityID]
		evaluatedAssignments[index] = tasks[ordinal].assignment
	}
	return compactPRISMStateFromEvaluation(state, evaluatedAssignments, clock, cost), nil
}

func prismStartReadyTasks(
	tasks []prismEvaluationTask,
	events *prismEvaluationTaskQueue,
	clock float64,
	resourceStart map[string]float64,
	resourceFinish map[string]float64,
	resourceUsed map[string]bool,
) bool {
	started := false
	for index := range tasks {
		task := &tasks[index]
		if task.started || task.remainingInputs > 0 || !task.laneReady {
			continue
		}
		task.started = true
		started = true
		overhead := prismMetadataNumber(task.assignment.Metadata, "bootOverheadSeconds") +
			prismMetadataNumber(task.assignment.Metadata, "containerOverheadSeconds")
		task.assignment.PredictedReadyAt = task.dataReadyAt
		task.assignment.PredictedStartAt = clock + overhead
		task.assignment.PredictedFinishAt = task.assignment.PredictedStartAt + task.assignment.PredictedRuntimeSeconds
		task.assignment.PredictedTransferSeconds = task.transferSeconds
		if task.assignment.Metadata == nil {
			task.assignment.Metadata = map[string]any{}
		}
		task.assignment.Metadata["queueSeconds"] = math.Max(0, clock-task.dataReadyAt)
		task.assignment.Metadata["networkContentionModel"] = "simgrid-shared-link-events"
		heap.Push(events, prismEvaluationTaskEvent{activity: index, finishAt: task.assignment.PredictedFinishAt})
		resourceID := task.assignment.ResourceID
		if !resourceUsed[resourceID] {
			resourceStart[resourceID] = clock
			resourceUsed[resourceID] = true
		}
		resourceFinish[resourceID] = math.Max(resourceFinish[resourceID], task.assignment.PredictedFinishAt)
	}
	return started
}

func prismEvaluationFlowRates(flows []prismEvaluationFlow, clock float64) ([]float64, float64) {
	rates := make([]float64, len(flows))
	linkUsers := make(map[string]int)
	nextAt := math.Inf(1)
	for index, flow := range flows {
		if !flow.active {
			continue
		}
		if flow.payloadAt > clock {
			nextAt = math.Min(nextAt, flow.payloadAt)
			continue
		}
		for _, hop := range flow.route.hops {
			linkUsers[hop.key]++
		}
		_ = index
	}
	for index, flow := range flows {
		if !flow.active || flow.payloadAt > clock {
			continue
		}
		rate := math.Inf(1)
		for _, hop := range flow.route.hops {
			users := math.Max(float64(linkUsers[hop.key]), 1)
			rate = math.Min(rate, hop.link.BandwidthBitsPerSecond/8/users)
		}
		if math.IsInf(rate, 1) || rate <= 0 {
			continue
		}
		rates[index] = rate
		nextAt = math.Min(nextAt, clock+flow.remaining/rate)
	}
	return rates, nextAt
}

func prismSatisfyEvaluationInput(task *prismEvaluationTask, readyAt float64, transferSeconds float64) {
	if task.remainingInputs > 0 {
		task.remainingInputs--
	}
	task.dataReadyAt = math.Max(task.dataReadyAt, readyAt)
	task.transferSeconds += transferSeconds
}

func compactPRISMStateFromEvaluation(
	state compactPRISMState,
	assignments []domain.PlanAssignment,
	makespan float64,
	cost float64,
) compactPRISMState {
	state.assignmentTrace = nil
	for index, assignment := range assignments {
		state.assignmentTrace = &compactPRISMAssignmentTrace{
			assignment: assignment,
			previous:   state.assignmentTrace,
			length:     index + 1,
		}
	}
	state.makespan = makespan
	state.cost = cost
	state.projectedMakespan = makespan
	state.projectedCost = cost
	return state
}
