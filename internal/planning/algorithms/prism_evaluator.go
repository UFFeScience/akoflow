package algorithms

import (
	"container/heap"
	"context"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type prismEvaluationDependency struct {
	consumer int
	bytes    int64
}

type prismEvaluationTask struct {
	assignment       domain.PlanAssignment
	resource         domain.Resource
	remainingInputs  int
	dataReadyAt      float64
	transferSeconds  float64
	laneReady        bool
	started          bool
	completed        bool
	baseRuntime      float64
	slowdown         float64
	remainingRuntime float64
	rate             float64
	rateUpdatedAt    float64
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

type prismEvaluationReadyQueue []int

func (queue prismEvaluationReadyQueue) Len() int           { return len(queue) }
func (queue prismEvaluationReadyQueue) Less(i, j int) bool { return queue[i] < queue[j] }
func (queue prismEvaluationReadyQueue) Swap(i, j int)      { queue[i], queue[j] = queue[j], queue[i] }
func (queue *prismEvaluationReadyQueue) Push(value any) {
	*queue = append(*queue, value.(int))
}
func (queue *prismEvaluationReadyQueue) Pop() any {
	old := *queue
	value := old[len(old)-1]
	*queue = old[:len(old)-1]
	return value
}

type prismEvaluationFlow struct {
	consumer  int
	source    string
	target    string
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
	if len(states) == 0 {
		return nil, nil
	}
	result := make([]compactPRISMState, len(states))
	type evaluationJob struct{ index int }
	jobs := make(chan evaluationJob)
	workerCount := min(len(states), max(1, runtime.GOMAXPROCS(0)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var workers sync.WaitGroup
	var firstErr error
	var errOnce sync.Once
	workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer workers.Done()
			for job := range jobs {
				if ctx.Err() != nil {
					continue
				}
				evaluated, err := evaluateCompleteCompactPRISMState(search, states[job.index])
				if err != nil {
					errOnce.Do(func() {
						firstErr = err
						cancel()
					})
					continue
				}
				result[job.index] = evaluated
			}
		}()
	}
	for index := range states {
		if ctx.Err() != nil {
			break
		}
		jobs <- evaluationJob{index: index}
	}
	close(jobs)
	workers.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return result, nil
}

func evaluateCompleteCompactPRISMState(
	search compactPRISMContext,
	state compactPRISMState,
) (compactPRISMState, error) {
	assignments := compactPRISMAssignments(search, state)
	if len(assignments) != len(search.activities) {
		return state, nil
	}
	resourceByID := make(map[string]domain.Resource, len(search.resources))
	for _, resource := range search.resources {
		resourceByID[resource.resource.ID] = resource.resource
	}
	tasks := make([]prismEvaluationTask, len(search.activities))
	assignmentOrdinal := make(map[string]int, len(assignments))
	for _, assignment := range assignments {
		assignment.Metadata = clonePRISMEvaluationMetadata(assignment.Metadata)
		ordinal, exists := search.activityOrdinal[assignment.ActivityID]
		if !exists {
			return state, fmt.Errorf("evaluate PRISM candidate: unknown activity %q", assignment.ActivityID)
		}
		resource, exists := resourceByID[assignment.ResourceID]
		if !exists {
			return state, fmt.Errorf("evaluate PRISM candidate: unknown resource %q", assignment.ResourceID)
		}
		tasks[ordinal] = prismEvaluationTask{assignment: assignment, resource: resource, baseRuntime: search.durations[ordinal][compactPRISMResourceOrdinal(search, resource.ID)], slowdown: 1}
		assignmentOrdinal[assignment.ActivityID] = ordinal
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
		sort.Slice(lane, func(left, right int) bool {
			return tasks[lane[left]].assignment.OrderOnResource <
				tasks[lane[right]].assignment.OrderOnResource
		})
		for index := 1; index < len(lane); index++ {
			tasks[lane[index]].laneReady = false
			laneSuccessor[lane[index-1]] = lane[index]
		}
	}

	clock := 0.0
	completed := 0
	taskEvents := &prismEvaluationTaskQueue{}
	heap.Init(taskEvents)
	readyTasks := &prismEvaluationReadyQueue{}
	heap.Init(readyTasks)
	for index := range tasks {
		if tasks[index].remainingInputs == 0 && tasks[index].laneReady {
			heap.Push(readyTasks, index)
		}
	}
	flows := make([]prismEvaluationFlow, 0, len(search.request.Workflow.Dependencies))
	resourceStart := make(map[string]float64)
	resourceFinish := make(map[string]float64)
	resourceUsed := make(map[string]bool)
	activeByResource := make(map[string]map[int]bool)
	transferCost := 0.0

	for completed < len(tasks) {
		started := prismStartReadyTasks(search, tasks, readyTasks, taskEvents, clock, resourceStart, resourceFinish, resourceUsed, activeByResource)
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
			if tasks[event.activity].completed || event.finishAt+1e-9 < tasks[event.activity].assignment.PredictedFinishAt {
				continue
			}
			resourceID := tasks[event.activity].assignment.ResourceID
			prismAdvanceActiveTasks(tasks, activeByResource[resourceID], clock)
			if tasks[event.activity].remainingRuntime > 1e-9 {
				prismRecomputePriorityRates(search, tasks, activeByResource[resourceID], taskEvents, clock)
				continue
			}
			tasks[event.activity].completed = true
			delete(activeByResource[resourceID], event.activity)
			prismRecomputePriorityRates(search, tasks, activeByResource[resourceID], taskEvents, clock)
			completed++
			if successor := laneSuccessor[event.activity]; successor >= 0 {
				tasks[successor].laneReady = true
				prismQueueEvaluationTaskIfReady(tasks, readyTasks, successor)
			}
			for _, dependency := range outgoing[event.activity] {
				producer := tasks[event.activity]
				consumer := &tasks[dependency.consumer]
				if dependency.bytes <= 0 || producer.assignment.ResourceID == consumer.assignment.ResourceID {
					prismSatisfyEvaluationInput(consumer, clock, 0)
					prismQueueEvaluationTaskIfReady(tasks, readyTasks, dependency.consumer)
					continue
				}
				route, exists := search.router.route(
					producer.assignment.ResourceID,
					consumer.assignment.ResourceID,
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
					consumer:  dependency.consumer,
					source:    producer.assignment.ResourceID,
					target:    consumer.assignment.ResourceID,
					bytes:     dependency.bytes,
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
			prismQueueEvaluationTaskIfReady(tasks, readyTasks, flows[index].consumer)
		}
		// Only active transfers affect future bandwidth allocation. Keeping every
		// completed flow made each later event rescan the entire dependency
		// history, turning large DAG evaluation into quadratic work.
		activeFlows := flows[:0]
		for _, flow := range flows {
			if flow.active {
				activeFlows = append(activeFlows, flow)
			}
		}
		flows = activeFlows
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

func clonePRISMEvaluationMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func prismStartReadyTasks(
	search compactPRISMContext,
	tasks []prismEvaluationTask,
	ready *prismEvaluationReadyQueue,
	events *prismEvaluationTaskQueue,
	clock float64,
	resourceStart map[string]float64,
	resourceFinish map[string]float64,
	resourceUsed map[string]bool,
	activeByResource map[string]map[int]bool,
) bool {
	started := false
	for ready.Len() > 0 {
		index := heap.Pop(ready).(int)
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
		resourceID := task.assignment.ResourceID
		prismAdvanceActiveTasks(tasks, activeByResource[resourceID], clock)
		task.remainingRuntime = task.baseRuntime
		task.rate = 1
		task.rateUpdatedAt = task.assignment.PredictedStartAt
		task.assignment.PredictedRuntimeSeconds = task.baseRuntime
		task.assignment.PredictedFinishAt = task.assignment.PredictedStartAt + task.baseRuntime
		task.assignment.PredictedTransferSeconds = task.transferSeconds
		if task.assignment.Metadata == nil {
			task.assignment.Metadata = map[string]any{}
		}
		task.assignment.Metadata["queueSeconds"] = math.Max(0, clock-task.dataReadyAt)
		task.assignment.Metadata["networkContentionModel"] = "simgrid-shared-link-events"
		task.assignment.Metadata["cpuPriorityWeight"] = 1.0
		if activeByResource[resourceID] == nil {
			activeByResource[resourceID] = map[int]bool{}
		}
		activeByResource[resourceID][index] = true
		prismRecomputePriorityRates(search, tasks, activeByResource[resourceID], events, clock)
		if !resourceUsed[resourceID] {
			resourceStart[resourceID] = clock
			resourceUsed[resourceID] = true
		}
		resourceFinish[resourceID] = math.Max(resourceFinish[resourceID], task.assignment.PredictedFinishAt)
	}
	return started
}

func compactPRISMResourceOrdinal(search compactPRISMContext, resourceID string) int {
	for ordinal, resource := range search.resources {
		if resource.resource.ID == resourceID {
			return ordinal
		}
	}
	return 0
}

func prismInterferencePriority(search compactPRISMContext, affected, interferer int) (float64, bool) {
	if affected < 0 || affected >= len(search.interference) || search.interference[affected] == nil {
		return 1, false
	}
	priority, exists := search.interference[affected][interferer]
	return math.Max(priority, 1e-12), exists
}

func prismAdvanceActiveTasks(tasks []prismEvaluationTask, active map[int]bool, clock float64) {
	for index := range active {
		task := &tasks[index]
		delta := math.Max(0, clock-task.rateUpdatedAt)
		task.remainingRuntime = math.Max(0, task.remainingRuntime-delta*task.rate)
		task.rateUpdatedAt = clock
	}
}

func prismRecomputePriorityRates(search compactPRISMContext, tasks []prismEvaluationTask, active map[int]bool, events *prismEvaluationTaskQueue, clock float64) {
	if len(active) == 0 {
		return
	}
	weights := make(map[int]float64, len(active))
	total := 0.0
	for index := range active {
		weight := math.Inf(1)
		for peer := range active {
			if peer == index {
				continue
			}
			if candidate, exists := prismInterferencePriority(search, index, peer); exists {
				weight = math.Min(weight, candidate)
			}
		}
		if math.IsInf(weight, 1) {
			weight = 1
		}
		weights[index] = weight
		total += weight
	}
	for index := range active {
		task := &tasks[index]
		capacity := math.Max(1, float64(task.resource.CPUCores))
		task.rate = math.Min(1, capacity*weights[index]/math.Max(total, 1e-12))
		task.slowdown = 1 / math.Max(task.rate, 1e-12)
		task.rateUpdatedAt = clock
		task.assignment.PredictedFinishAt = clock + task.remainingRuntime/math.Max(task.rate, 1e-12)
		task.assignment.PredictedRuntimeSeconds = task.assignment.PredictedFinishAt - task.assignment.PredictedStartAt
		if task.assignment.Metadata == nil {
			task.assignment.Metadata = map[string]any{}
		}
		task.assignment.Metadata["cpuPriorityWeight"] = weights[index]
		task.assignment.Metadata["interferenceSlowdown"] = task.slowdown
		heap.Push(events, prismEvaluationTaskEvent{activity: index, finishAt: task.assignment.PredictedFinishAt})
	}
}

func prismQueueEvaluationTaskIfReady(
	tasks []prismEvaluationTask,
	ready *prismEvaluationReadyQueue,
	index int,
) {
	task := &tasks[index]
	if !task.started && task.remainingInputs == 0 && task.laneReady {
		heap.Push(ready, index)
	}
}

func prismEvaluationFlowRates(flows []prismEvaluationFlow, clock float64) ([]float64, float64) {
	rates := make([]float64, len(flows))
	linkUsers := make(map[string]int)
	sourceUsers := make(map[string]int)
	targetUsers := make(map[string]int)
	pairUsers := make(map[string]int)
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
		sourceUsers[flow.source]++
		targetUsers[flow.target]++
		pairUsers[flow.source+"\x00"+flow.target]++
		_ = index
	}
	for index, flow := range flows {
		if !flow.active || flow.payloadAt > clock {
			continue
		}
		rate := math.Inf(1)
		// Distinct routes can still contend at their sending or receiving
		// resource. Count the flow itself once, even when source and target are
		// shared by the same set of active flows.
		endpointUsers := sourceUsers[flow.source] + targetUsers[flow.target] -
			pairUsers[flow.source+"\x00"+flow.target]
		for _, hop := range flow.route.hops {
			users := math.Max(float64(max(linkUsers[hop.key], endpointUsers)), 1)
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
	state.evaluatedAssignments = append([]domain.PlanAssignment(nil), assignments...)
	state.queueSeconds = 0
	state.transferSeconds = 0
	state.networkCost = 0
	state.usedResourceCount = 0
	usedResources := make(map[string]struct{}, len(assignments))
	for _, assignment := range assignments {
		state.queueSeconds += prismMetadataNumber(assignment.Metadata, "queueSeconds")
		state.transferSeconds += assignment.PredictedTransferSeconds
		state.networkCost += prismMetadataNumber(assignment.Metadata, "transferCost")
		usedResources[assignment.ResourceID] = struct{}{}
	}
	state.usedResourceCount = len(usedResources)
	state.makespan = makespan
	state.cost = cost
	state.projectedMakespan = makespan
	state.projectedCost = cost
	return state
}
