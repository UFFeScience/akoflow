package planner

import (
	"container/heap"
	"fmt"
	"math"
	"sort"
	"strings"
)

type Algorithm string

const (
	MemoryAware Algorithm = "ako-memory"
	FIFO        Algorithm = "fifo"
)

type Options struct {
	Algorithm             Algorithm
	PlanID                string
	Alpha                 float64
	DefaultRuntimeSeconds float64
}

type interval struct {
	start       float64
	finish      float64
	cpu         float64
	memoryBytes int64
}

type candidate struct {
	resource   Resource
	readyAt    float64
	start      float64
	finish     float64
	runtime    float64
	transfer   float64
	freeMemory int64
	score      float64
	coreIndex  int
}

type state struct {
	assignments  []Assignment
	byActivity   map[string]Assignment
	intervals    map[string][]interval
	resourceRuns map[string]int
	coreOrders   map[string]int
	cost         float64
	makespan     float64
}

func ValidateInput(input Input) error {
	if input.Workflow.ID == "" {
		return fmt.Errorf("workflow.id is required")
	}
	if input.ExecutionScope.ID == "" {
		return fmt.Errorf("executionScope.id is required")
	}
	if input.NetworkTopology.ID == "" {
		return fmt.Errorf("networkTopology.id is required")
	}
	if input.NetworkTopology.ExecutionScopeID != input.ExecutionScope.ID {
		return fmt.Errorf("topology scope %q does not match execution scope %q", input.NetworkTopology.ExecutionScopeID, input.ExecutionScope.ID)
	}
	if len(input.Workflow.Activities) == 0 {
		return fmt.Errorf("workflow has no activities")
	}
	return nil
}

func BuildPlan(input Input, options Options) (ImportEnvelope, error) {
	if err := ValidateInput(input); err != nil {
		return ImportEnvelope{}, err
	}
	if options.PlanID == "" {
		return ImportEnvelope{}, fmt.Errorf("plan ID is required")
	}
	if options.Alpha < 0 || options.Alpha > 1 {
		return ImportEnvelope{}, fmt.Errorf("alpha must be between 0 and 1")
	}
	if options.DefaultRuntimeSeconds <= 0 {
		options.DefaultRuntimeSeconds = 1
	}
	ordered, err := topologicalOrder(input.Workflow)
	if err != nil {
		return ImportEnvelope{}, err
	}
	resources := scopedResources(input)
	if len(resources) == 0 {
		return ImportEnvelope{}, fmt.Errorf("execution scope has no schedulable resources")
	}
	maximumMemory := int64(1)
	for _, resource := range resources {
		if resource.MemoryBytes > maximumMemory {
			maximumMemory = resource.MemoryBytes
		}
	}
	schedule := state{
		byActivity:   map[string]Assignment{},
		intervals:    map[string][]interval{},
		resourceRuns: map[string]int{},
		coreOrders:   map[string]int{},
	}
	for sequence, activity := range ordered {
		choices := make([]candidate, 0, len(resources))
		for _, resource := range resources {
			choice, feasible := evaluateCandidate(input, schedule, activity, resource, maximumMemory, options)
			if feasible {
				choices = append(choices, choice)
			}
		}
		if len(choices) == 0 {
			return ImportEnvelope{}, fmt.Errorf("activity %q does not fit any resource in scope", activity.ID)
		}
		selected := selectCandidate(choices, options.Algorithm)
		assignment := makeAssignment(input, schedule, activity, selected, options, sequence)
		schedule.assignments = append(schedule.assignments, assignment)
		schedule.byActivity[activity.ID] = assignment
		schedule.intervals[selected.resource.ID] = append(schedule.intervals[selected.resource.ID], interval{
			start:       selected.start,
			finish:      selected.finish,
			cpu:         requiredCPU(activity),
			memoryBytes: requiredMemory(input, activity, selected.resource),
		})
		schedule.resourceRuns[selected.resource.ID]++
		schedule.coreOrders[assignment.CoreID]++
		schedule.cost += assignment.PredictedCost
		if assignment.PredictedFinishAt > schedule.makespan {
			schedule.makespan = assignment.PredictedFinishAt
		}
	}
	feasible := (input.DeadlineSeconds <= 0 || schedule.makespan <= input.DeadlineSeconds) &&
		(input.Budget <= 0 || schedule.cost <= input.Budget)
	plan := SchedulePlan{
		ID:                options.PlanID,
		WorkflowVersionID: input.Workflow.ID,
		ExecutionScopeID:  input.ExecutionScope.ID,
		NetworkTopologyID: input.NetworkTopology.ID,
		Source:            "imported",
		Algorithm:         string(options.Algorithm),
		AlgorithmVersion:  "1",
		Objective:         objective(options.Algorithm),
		DeadlineSeconds:   input.DeadlineSeconds,
		Budget:            input.Budget,
		Predicted: PredictedMetrics{
			MakespanSeconds: schedule.makespan,
			Cost:            schedule.cost,
			Feasible:        feasible,
		},
		Assignments: schedule.assignments,
		Metadata: map[string]any{
			"generator":             "akoflow-other-algorithms",
			"defaultRuntimeSeconds": options.DefaultRuntimeSeconds,
		},
	}
	if options.Algorithm == MemoryAware {
		plan.Metadata["alpha"] = options.Alpha
		plan.Metadata["score"] = "alpha*(1/runtime)+(1-alpha)*((freeMemory-requiredMemory)/maximumMemory)"
	}
	return ImportEnvelope{Plan: plan}, nil
}

func scopedResources(input Input) []Resource {
	allowed := map[string]bool{}
	for _, id := range input.ExecutionScope.EnvironmentVersionIDs {
		allowed[id] = true
	}
	var resources []Resource
	for _, resource := range input.Resources {
		if resource.Schedulable && allowed[resource.EnvironmentVersionID] {
			resources = append(resources, resource)
		}
	}
	sort.SliceStable(resources, func(i, j int) bool { return resources[i].ID < resources[j].ID })
	return resources
}

func topologicalOrder(workflow WorkflowVersion) ([]Activity, error) {
	byID := map[string]Activity{}
	index := map[string]int{}
	indegree := map[string]int{}
	successors := map[string][]string{}
	for position, activity := range workflow.Activities {
		if activity.ID == "" {
			return nil, fmt.Errorf("workflow contains an activity without an ID")
		}
		byID[activity.ID] = activity
		index[activity.ID] = position
		indegree[activity.ID] = 0
	}
	for _, dependency := range workflow.Dependencies {
		if _, ok := byID[dependency.ActivityID]; !ok {
			return nil, fmt.Errorf("dependency references unknown activity %q", dependency.ActivityID)
		}
		if _, ok := byID[dependency.DependsOnActivityID]; !ok {
			return nil, fmt.Errorf("dependency references unknown predecessor %q", dependency.DependsOnActivityID)
		}
		indegree[dependency.ActivityID]++
		successors[dependency.DependsOnActivityID] = append(successors[dependency.DependsOnActivityID], dependency.ActivityID)
	}
	ready := make([]string, 0)
	for _, activity := range workflow.Activities {
		if indegree[activity.ID] == 0 {
			ready = append(ready, activity.ID)
		}
	}
	var ordered []Activity
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, byID[id])
		for _, successor := range successors[id] {
			indegree[successor]--
			if indegree[successor] == 0 {
				ready = append(ready, successor)
			}
		}
		sort.SliceStable(ready, func(i, j int) bool { return index[ready[i]] < index[ready[j]] })
	}
	if len(ordered) != len(workflow.Activities) {
		return nil, fmt.Errorf("workflow dependencies contain a cycle")
	}
	return ordered, nil
}

func evaluateCandidate(input Input, schedule state, activity Activity, resource Resource, maximumMemory int64, options Options) (candidate, bool) {
	if selector, _ := activity.Metadata["resourceSelector"].(string); selector != "" && selector != resource.ID {
		return candidate{}, false
	}
	cpu := requiredCPU(activity)
	memory := requiredMemory(input, activity, resource)
	capacity := resource.CPUCapacity
	if capacity <= 0 {
		capacity = float64(max(resource.CPUCores, 1))
	}
	if cpu > capacity || memory > resource.MemoryBytes {
		return candidate{}, false
	}
	readyAt, transfer := dependencyReadyTime(input, schedule, activity, resource.ID)
	runtime := activityRuntime(input, activity, resource, options.DefaultRuntimeSeconds)
	duration := runtime + resource.ContainerOverhead
	if schedule.resourceRuns[resource.ID] == 0 {
		duration += resource.BootOverheadSeconds
	}
	start, freeMemory, coreIndex := earliestFit(schedule.intervals[resource.ID], readyAt, duration, cpu, memory, capacity, resource.MemoryBytes, resource.CPUCores)
	score := options.Alpha*(1/math.Max(runtime, 1e-9)) +
		(1-options.Alpha)*(float64(freeMemory-memory)/float64(maximumMemory))
	return candidate{
		resource:   resource,
		readyAt:    readyAt,
		start:      start,
		finish:     start + duration,
		runtime:    runtime,
		transfer:   transfer,
		freeMemory: freeMemory,
		score:      score,
		coreIndex:  coreIndex,
	}, true
}

func selectCandidate(choices []candidate, algorithm Algorithm) candidate {
	best := choices[0]
	for _, choice := range choices[1:] {
		switch algorithm {
		case MemoryAware:
			if choice.score > best.score || (choice.score == best.score && earlier(choice, best)) {
				best = choice
			}
		default:
			if earlier(choice, best) {
				best = choice
			}
		}
	}
	return best
}

func earlier(left, right candidate) bool {
	if left.start != right.start {
		return left.start < right.start
	}
	if left.finish != right.finish {
		return left.finish < right.finish
	}
	return left.resource.ID < right.resource.ID
}

func earliestFit(intervals []interval, readyAt, duration, cpu float64, memory int64, cpuCapacity float64, memoryCapacity int64, cores int) (float64, int64, int) {
	candidates := []float64{readyAt}
	for _, item := range intervals {
		if item.finish >= readyAt {
			candidates = append(candidates, item.finish)
		}
	}
	sort.Float64s(candidates)
	for _, start := range candidates {
		finish := start + duration
		usedCPU := 0.0
		usedMemory := int64(0)
		active := 0
		for _, item := range intervals {
			if item.start < finish && start < item.finish {
				usedCPU += item.cpu
				usedMemory += item.memoryBytes
				active++
			}
		}
		if usedCPU+cpu <= cpuCapacity && usedMemory+memory <= memoryCapacity {
			coreCount := max(cores, 1)
			return start, memoryCapacity - usedMemory, active % coreCount
		}
	}
	return readyAt, memoryCapacity, 0
}

func dependencyReadyTime(input Input, schedule state, activity Activity, resourceID string) (float64, float64) {
	readyAt := 0.0
	totalTransfer := 0.0
	for _, dependency := range input.Workflow.Dependencies {
		if dependency.ActivityID != activity.ID {
			continue
		}
		predecessor := schedule.byActivity[dependency.DependsOnActivityID]
		bytes := dependencyBytes(input.Workflow, dependency.DependsOnActivityID, activity.ID)
		transfer := shortestTransfer(input.NetworkTopology, predecessor.ResourceID, resourceID, bytes)
		totalTransfer += transfer
		if predecessor.PredictedFinishAt+transfer > readyAt {
			readyAt = predecessor.PredictedFinishAt + transfer
		}
	}
	return readyAt, totalTransfer
}

func dependencyBytes(workflow WorkflowVersion, producer, consumer string) int64 {
	var total int64
	for _, dependency := range workflow.DataDependencies {
		if dependency.ProducerActivityID == producer && dependency.ConsumerActivityID == consumer {
			total += dependency.SizeBytes
		}
	}
	return total
}

type graphEdge struct {
	target string
	time   float64
}

type distanceItem struct {
	node     string
	distance float64
	index    int
}

type distanceQueue []*distanceItem

func (queue distanceQueue) Len() int           { return len(queue) }
func (queue distanceQueue) Less(i, j int) bool { return queue[i].distance < queue[j].distance }
func (queue distanceQueue) Swap(i, j int) {
	queue[i], queue[j] = queue[j], queue[i]
	queue[i].index, queue[j].index = i, j
}
func (queue *distanceQueue) Push(value any) {
	item := value.(*distanceItem)
	item.index = len(*queue)
	*queue = append(*queue, item)
}
func (queue *distanceQueue) Pop() any {
	old := *queue
	item := old[len(old)-1]
	*queue = old[:len(old)-1]
	return item
}

func shortestTransfer(topology NetworkTopology, source, target string, bytes int64) float64 {
	if source == target || bytes <= 0 {
		return 0
	}
	graph := map[string][]graphEdge{}
	for _, link := range topology.Links {
		if link.BandwidthBitsPerSecond <= 0 {
			continue
		}
		time := link.LatencySeconds + float64(bytes)/(link.BandwidthBitsPerSecond/8)
		graph[link.SourceResourceID] = append(graph[link.SourceResourceID], graphEdge{target: link.TargetResourceID, time: time})
		if link.Bidirectional {
			graph[link.TargetResourceID] = append(graph[link.TargetResourceID], graphEdge{target: link.SourceResourceID, time: time})
		}
	}
	distances := map[string]float64{source: 0}
	queue := distanceQueue{&distanceItem{node: source, distance: 0}}
	heap.Init(&queue)
	for queue.Len() > 0 {
		current := heap.Pop(&queue).(*distanceItem)
		if current.node == target {
			return current.distance
		}
		if known := distances[current.node]; current.distance > known {
			continue
		}
		for _, edge := range graph[current.node] {
			candidate := current.distance + edge.time
			known, exists := distances[edge.target]
			if !exists || candidate < known {
				distances[edge.target] = candidate
				heap.Push(&queue, &distanceItem{node: edge.target, distance: candidate})
			}
		}
	}
	return 0
}

func activityRuntime(input Input, activity Activity, resource Resource, fallback float64) float64 {
	for _, profile := range input.ActivityProfiles {
		if profile.ActivityTypeID == activity.ActivityTypeID && profile.ResourceID == resource.ID && profile.RuntimeSeconds > 0 {
			return profile.RuntimeSeconds
		}
	}
	if activity.Simulation != nil && activity.Simulation.FLOPs > 0 {
		if flops := number(resource.Metadata, "flopsPerSecond"); flops > 0 {
			return activity.Simulation.FLOPs / flops
		}
	}
	base := 0.0
	if activity.Simulation != nil {
		base = activity.Simulation.DurationSeconds
	}
	if base <= 0 {
		base = number(activity.Metadata, "baseRuntimeSeconds")
	}
	if base <= 0 {
		base = fallback
	}
	speedup := resource.ComputeSpeedup
	if speedup <= 0 {
		speedup = 1
	}
	return base / speedup
}

func requiredMemory(input Input, activity Activity, resource Resource) int64 {
	memory := activity.Resources.MemoryBytes
	for _, profile := range input.ActivityProfiles {
		if profile.ActivityTypeID == activity.ActivityTypeID && profile.ResourceID == resource.ID && profile.PeakMemoryBytes > memory {
			memory = profile.PeakMemoryBytes
		}
	}
	return memory
}

func requiredCPU(activity Activity) float64 {
	if activity.Resources.CPU > 0 {
		return activity.Resources.CPU
	}
	return 1
}

func makeAssignment(input Input, schedule state, activity Activity, choice candidate, options Options, sequence int) Assignment {
	coreID := fmt.Sprintf("%s-core-%d", choice.resource.ID, choice.coreIndex+1)
	order := schedule.coreOrders[coreID]
	return Assignment{
		ID:                       fmt.Sprintf("%s-assignment-%04d", options.PlanID, sequence+1),
		PlanID:                   options.PlanID,
		ActivityID:               activity.ID,
		ResourceID:               choice.resource.ID,
		CoreID:                   coreID,
		SlotID:                   "default",
		OrderOnResource:          order,
		Priority:                 activity.Priority,
		PredictedReadyAt:         choice.readyAt,
		PredictedStartAt:         choice.start,
		PredictedFinishAt:        choice.finish,
		PredictedRuntimeSeconds:  choice.runtime,
		PredictedTransferSeconds: choice.transfer,
		PredictedCost:            choice.runtime * choice.resource.PricePerSecond,
		Metadata: map[string]any{
			"algorithm":           string(options.Algorithm),
			"requestedCPU":        requiredCPU(activity),
			"requiredMemoryBytes": requiredMemory(input, activity, choice.resource),
			"freeMemoryAtStart":   choice.freeMemory,
			"score":               choice.score,
		},
	}
}

func number(values map[string]any, key string) float64 {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case uint64:
		return float64(value)
	}
	return 0
}

func objective(algorithm Algorithm) string {
	if algorithm == MemoryAware {
		return "weighted-time-memory"
	}
	return "fifo"
}

func ParseAlgorithm(value string) (Algorithm, error) {
	switch strings.ToLower(value) {
	case "memory", "memory-aware", "ako-memory":
		return MemoryAware, nil
	case "fifo":
		return FIFO, nil
	default:
		return "", fmt.Errorf("unknown algorithm %q", value)
	}
}
