package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// MOHEFT implements Algorithm 2 of Durillo, Fard and Prodan (CloudCom 2012).
// Its frontier is ordered by Pareto dominance, then NSGA-II crowding distance.
// Task timing uses Akoflow's CPU lanes and fixed-link transfer estimates.
// Prices follow the paper's task-additive model, not PRISM's VM active window.
func Schedule(input Input) ([]Plan, error) {
	model, err := newMOModel(input)
	if err != nil {
		return nil, err
	}
	k := 25
	if raw, ok := input.Configuration["candidateCount"]; ok {
		number, ok := raw.(float64)
		if !ok || math.Trunc(number) != number || number < 1 || number > 1000 {
			return nil, fmt.Errorf("candidateCount must be an integer between 1 and 1000")
		}
		k = int(number)
	}
	frontier := []moState{model.empty()}
	for _, task := range model.order {
		next := make([]moState, 0, len(frontier)*len(model.resources))
		for _, state := range frontier {
			for resource := range model.resources {
				candidate, ok := model.place(state, task, resource)
				if ok {
					next = append(next, candidate)
				}
			}
		}
		if len(next) == 0 {
			return nil, fmt.Errorf("no feasible placement for activity %q", model.tasks[task].ID)
		}
		frontier = moSelect(next, k)
	}
	plans := make([]Plan, 0, len(frontier))
	for i, state := range frontier {
		p := model.plan(state, i)
		p.Metadata["candidateCount"] = len(frontier)
		plans = append(plans, p)
	}
	return plans, nil
}

type moLink struct {
	SourceResourceID, TargetResourceID                   string
	BandwidthBitsPerSecond, LatencySeconds, PricePerByte float64
	Bidirectional                                        bool
}
type moProfile struct {
	ActivityTypeID, ResourceID, ModelVersion string
	RuntimeSeconds                           float64
}
type moData struct {
	ProducerActivityID, ConsumerActivityID string
	SizeBytes                              int64
}
type moEdge struct {
	task  int
	bytes int64
}
type moRecord struct {
	assignment Assignment
	resource   int
}
type moInterval struct{ start, finish float64 }
type moMemory struct {
	start, finish float64
	bytes         int64
}
type moBlock [64]*moRecord

// Persistent, chunked placement vectors avoid copying the complete DAG on every expansion.
type moState struct {
	blocks         []*moBlock
	slots          [][][]moInterval
	memory         [][]moMemory
	makespan, cost float64
	signature      uint64
}

func (s moState) get(task int) *moRecord {
	if s.blocks[task/64] == nil {
		return nil
	}
	return s.blocks[task/64][task%64]
}

type moModel struct {
	input             Input
	tasks             []Activity
	resources         []Resource
	parents, children [][]moEdge
	order             []int
	links             map[[2]string]moLink
	runtimes          [][]float64
	lanes             []int
	topologyID        string
	index             map[string]int
	memoryCheck       []bool
}

func newMOModel(input Input) (*moModel, error) {
	m := &moModel{input: input, tasks: input.Request.Workflow.Activities, links: map[[2]string]moLink{}}
	if !moFinite(input.Request.Budget) || input.Request.Budget < 0 || !moFinite(input.Request.DeadlineSeconds) || input.Request.DeadlineSeconds < 0 {
		return nil, fmt.Errorf("budget and deadline must be finite and nonnegative")
	}
	if len(m.tasks) == 0 {
		return nil, fmt.Errorf("workflow has no activities")
	}
	if err := m.loadResources(); err != nil {
		return nil, err
	}
	if err := m.loadGraph(); err != nil {
		return nil, err
	}
	if err := m.loadRuntimes(); err != nil {
		return nil, err
	}
	order, err := m.rankOrder()
	if err != nil {
		return nil, err
	}
	m.order = order
	return m, nil
}
func (m *moModel) empty() moState {
	s := moState{blocks: make([]*moBlock, (len(m.tasks)+63)/64), slots: make([][][]moInterval, len(m.resources)),
		memory: make([][]moMemory, len(m.resources)), signature: 14695981039346656037}
	for r := range m.resources {
		s.slots[r] = make([][]moInterval, m.lanes[r])
	}
	return s
}
func (m *moModel) rankOrder() ([]int, error) {
	indegree := make([]int, len(m.tasks))
	for i := range indegree {
		indegree[i] = len(m.parents[i])
	}
	topo := make([]int, 0, len(m.tasks))
	ready := []int{}
	for i, n := range indegree {
		if n == 0 {
			ready = append(ready, i)
		}
	}
	for len(ready) > 0 {
		sort.Slice(ready, func(i, j int) bool { return m.tasks[ready[i]].ID < m.tasks[ready[j]].ID })
		a := ready[0]
		ready = ready[1:]
		topo = append(topo, a)
		for _, e := range m.children[a] {
			indegree[e.task]--
			if indegree[e.task] == 0 {
				ready = append(ready, e.task)
			}
		}
	}
	if len(topo) != len(m.tasks) {
		return nil, fmt.Errorf("workflow contains a cycle")
	}
	rank := make([]float64, len(m.tasks))
	position := make([]int, len(m.tasks))
	for i := len(topo) - 1; i >= 0; i-- {
		a := topo[i]
		position[a] = i
		avg := 0.0
		count := 0
		for r := range m.resources {
			if m.fits(a, r) {
				avg += m.runtimes[a][r]
				count++
			}
		}
		if count == 0 {
			return nil, fmt.Errorf("activity %s has no feasible resource", m.tasks[a].ID)
		}
		avg /= float64(count)
		tail := 0.0
		for _, e := range m.children[a] {
			comm := 0.0
			pairs := 0
			for r, from := range m.resources {
				for q, to := range m.resources {
					if !m.fits(a, r) || !m.fits(e.task, q) {
						continue
					}
					seconds, ok := m.transfer(from.ID, to.ID, e.bytes)
					if ok {
						comm += seconds
						pairs++
					}
				}
			}
			if pairs > 0 {
				comm /= float64(pairs)
			}
			tail = math.Max(tail, comm+rank[e.task])
		}
		rank[a] = avg + tail
	}
	// Sorting by positive upward rank preserves precedence; position is the zero-edge tie break.
	order := append([]int(nil), topo...)
	sort.SliceStable(order, func(i, j int) bool {
		if rank[order[i]] != rank[order[j]] {
			return rank[order[i]] > rank[order[j]]
		}
		return position[order[i]] < position[order[j]]
	})
	return order, nil
}
func (m *moModel) fits(task, resource int) bool {
	a, r := m.tasks[task], m.resources[resource]
	selector, _ := a.Metadata["resourceSelector"].(string)
	return (selector == "" || selector == r.ID) && r.CPUCapacity >= a.Resources.CPU && r.MemoryBytes >= a.Resources.MemoryBytes
}
func (m *moModel) transfer(from, to string, size int64) (float64, bool) {
	if from == to || size == 0 {
		return 0, true
	}
	l, ok := m.links[[2]string{from, to}]
	if !ok {
		return 0, false
	}
	return l.LatencySeconds + float64(size)*8/l.BandwidthBitsPerSecond, true
}
func (m *moModel) place(s moState, task, resource int) (moState, bool) {
	if !m.fits(task, resource) {
		return moState{}, false
	}
	r := m.resources[resource]
	parentFinish, transferTotal, ingress, egress := 0.0, 0.0, 0.0, 0.0
	for _, e := range m.parents[task] {
		parent := s.get(e.task)
		if parent == nil {
			return moState{}, false
		}
		from := m.resources[parent.resource]
		seconds, ok := m.transfer(from.ID, r.ID, e.bytes)
		if !ok {
			return moState{}, false
		}
		parentFinish = math.Max(parentFinish, parent.assignment.PredictedFinishAt)
		transferTotal = math.Max(transferTotal, seconds)
		if from.ID != r.ID {
			size := float64(e.bytes)
			ingress += size * moPrice(r.Metadata, "ingressPricePerByte")
			egress += size * moPrice(from.Metadata, "egressPricePerByte")
		}
	}
	// Paper Eqs. 1-2 use the maximum transfer, after the latest predecessor.
	ready := parentFinish + transferTotal
	runtime := m.runtimes[task][resource]
	lane, start := m.selectLane(s, task, resource, ready, runtime)
	finish := start + runtime
	// Paper Eq. 4: CPU seconds + storage-time + remote ingress/egress. Bandwidth link tariffs do not replace the paper's ingress/egress prices.
	billed := runtime + transferTotal
	ownCost := billed*r.PricePerSecond + float64(m.tasks[task].Resources.StorageBytes)*billed*moPrice(r.Metadata, "storagePricePerByteSecond") + ingress
	record := &moRecord{resource: resource,
		assignment: Assignment{Metadata: map[string]any{"bootOverheadSeconds": float64(0),
			"resourceAvailableAtSeconds": r.BootOverheadSeconds,
			"containerOverheadSeconds":   r.ContainerOverheadSeconds},
			ActivityID: m.tasks[task].ID,
			Priority:   m.tasks[task].Priority,
			ResourceID: r.ID,
			CoreID: fmt.Sprintf("%s-core-%d",
				r.ID,
				lane+1),
			PredictedReadyAt:         ready,
			PredictedStartAt:         start,
			PredictedFinishAt:        finish,
			PredictedRuntimeSeconds:  runtime,
			PredictedTransferSeconds: transferTotal,
			PredictedCost:            ownCost + egress}}
	out := s
	out.blocks = append([]*moBlock(nil), s.blocks...)
	block := &moBlock{}
	if s.blocks[task/64] != nil {
		*block = *s.blocks[task/64]
	}
	block[task%64] = record
	out.blocks[task/64] = block
	out.slots = append([][][]moInterval(nil), s.slots...)
	out.slots[resource] = append([][]moInterval(nil), s.slots[resource]...)
	intervals := append([]moInterval(nil), s.slots[resource][lane]...)
	intervals = append(intervals, moInterval{start - r.ContainerOverheadSeconds, finish})
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].start < intervals[j].start })
	out.slots[resource][lane] = intervals
	if m.memoryCheck[resource] {
		out.memory = append([][]moMemory(nil), s.memory...)
		out.memory[resource] = append(append([]moMemory(nil), s.memory[resource]...), moMemory{start - r.ContainerOverheadSeconds, finish, m.tasks[task].Resources.MemoryBytes})
	}
	out.makespan = math.Max(s.makespan, finish)
	out.cost = s.cost + ownCost + egress
	out.signature = (s.signature ^ uint64(resource+1)) * 1099511628211
	return out, moFinite(out.cost) && moFinite(finish)
}
func (m *moModel) plan(s moState, index int) Plan {
	id := fmt.Sprintf("moheft-%d", index+1)
	assignments := make([]Assignment, 0, len(m.tasks))
	for resource := range m.resources {
		for lane := 0; lane < m.lanes[resource]; lane++ {
			items := []Assignment{}
			core := fmt.Sprintf("%s-core-%d", m.resources[resource].ID, lane+1)
			for _, task := range m.order {
				a := s.get(task).assignment
				if a.CoreID == core {
					items = append(items, a)
				}
			}
			sort.SliceStable(items, func(i, j int) bool { return items[i].PredictedStartAt < items[j].PredictedStartAt })
			for order, a := range items {
				a.PredictedCost = m.finalTaskCost(s, a.ActivityID)
				a.ID = id + "-" + a.ActivityID
				a.PlanID = id
				a.OrderOnResource = order
				assignments = append(assignments, a)
			}
		}
	}
	feasible := (m.input.Request.DeadlineSeconds <= 0 || s.makespan <= m.input.Request.DeadlineSeconds) && (m.input.Request.Budget <= 0 || s.cost <= m.input.Request.Budget)
	return Plan{ID: id,
		WorkflowVersionID: m.input.Request.Workflow.ID,
		ExecutionScopeID:  m.input.Request.ExecutionScope.ID,
		NetworkTopologyID: m.topologyID,
		Algorithm:         "moheft",
		Objective:         "pareto",
		DeadlineSeconds:   m.input.Request.DeadlineSeconds,
		Budget:            m.input.Request.Budget,
		Predicted: Metrics{MakespanSeconds: s.makespan,
			Cost:     s.cost,
			Feasible: feasible},
		Assignments: assignments,
		Metadata: map[string]any{"algorithmReference": "doi:10.1109/CloudCom.2012.6427573",
			"costModel":         "moheft-task-additive",
			"timingModel":       "akoflow-cpu-lanes-fixed-link",
			"interferenceAware": false}}
}
func moFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func moPrice(metadata map[string]any, key string) float64 {
	value, _ := metadata[key].(float64)
	if value < 0 || !moFinite(value) {
		return 0
	}
	return value
}
func moDominates(a, b moState) bool {
	return a.makespan <= b.makespan && a.cost <= b.cost && (a.makespan < b.makespan || a.cost < b.cost)
}
func moSelect(candidates []moState, k int) []moState {
	// Equal-objective mappings remain distinct: descendants may communicate differently.
	front := make([]moState, 0, len(candidates))
	for i, a := range candidates {
		dominated := false
		for j, b := range candidates {
			if i != j && moDominates(b, a) {
				dominated = true
				break
			}
		}
		if !dominated {
			front = append(front, a)
		}
	}
	if len(front) <= k {
		return front
	}
	distance := make([]float64, len(front))
	indices := make([]int, len(front))
	for i := range indices {
		indices[i] = i
	}
	for objective := 0; objective < 2; objective++ {
		value := func(i int) float64 {
			if objective == 0 {
				return front[i].makespan
			}
			return front[i].cost
		}
		sort.SliceStable(indices, func(i, j int) bool { return value(indices[i]) < value(indices[j]) })
		low, high := value(indices[0]), value(indices[len(indices)-1])
		if high == low {
			continue
		}
		distance[indices[0]] = math.Inf(1)
		distance[indices[len(indices)-1]] = math.Inf(1)
		for i := 1; i < len(indices)-1; i++ {
			distance[indices[i]] += (value(indices[i+1]) - value(indices[i-1])) / (high - low)
		}
	}
	sort.SliceStable(indices, func(i, j int) bool {
		a, b := indices[i], indices[j]
		if distance[a] != distance[b] {
			return distance[a] > distance[b]
		}
		if front[a].makespan != front[b].makespan {
			return front[a].makespan < front[b].makespan
		}
		if front[a].cost != front[b].cost {
			return front[a].cost < front[b].cost
		}
		return front[a].signature < front[b].signature
	})
	selected := make([]moState, 0, k)
	for _, i := range indices[:k] {
		selected = append(selected, front[i])
	}
	return selected
}

// moMemoryReady returns the first blocking allocation's release if RAM would overflow.
func moMemoryReady(usage []moMemory, start, finish float64, requested, capacity int64) float64 {
	if requested == 0 {
		return start
	}
	events := []float64{start}
	for _, item := range usage {
		if item.start > start && item.start < finish {
			events = append(events, item.start)
		}
	}
	sort.Float64s(events)
	for _, at := range events {
		used := requested
		next := math.Inf(1)
		for _, item := range usage {
			if item.start <= at && item.finish > at {
				if item.bytes > capacity-used {
					return moFirstRelease(usage, at)
				}
				used += item.bytes
				next = math.Min(next, item.finish)
			}
		}
		if used > capacity {
			return next
		}
	}
	return start
}
func moFirstRelease(usage []moMemory, at float64) float64 {
	next := math.Inf(1)
	for _, item := range usage {
		if item.start <= at && item.finish > at && item.bytes > 0 {
			next = math.Min(next, item.finish)
		}
	}
	return next
}

func (m *moModel) finalTaskCost(s moState, id string) float64 {
	task := m.index[id]
	placed := s.get(task)
	r := m.resources[placed.resource]
	billed := placed.assignment.PredictedRuntimeSeconds + placed.assignment.PredictedTransferSeconds
	cost := billed*r.PricePerSecond + float64(m.tasks[task].Resources.StorageBytes)*billed*moPrice(r.Metadata, "storagePricePerByteSecond")
	for _, e := range m.parents[task] {
		if s.get(e.task).resource != placed.resource {
			cost += float64(e.bytes) * moPrice(r.Metadata, "ingressPricePerByte")
		}
	}
	for _, e := range m.children[task] {
		if s.get(e.task).resource != placed.resource {
			cost += float64(e.bytes) * moPrice(r.Metadata, "egressPricePerByte")
		}
	}
	return cost
}

func (m *moModel) loadResources() error {
	allowed := map[string]bool{}
	for _, id := range m.input.Request.ExecutionScope.EnvironmentVersionIDs {
		allowed[id] = true
	}
	for _, r := range m.input.Request.Resources {
		if r.Type == "batch_queue" || r.Type == "hpc_partition" || r.Type == "slurm_reservation" {
			continue
		}
		if !r.Schedulable || !allowed[r.EnvironmentVersionID] {
			continue
		}
		if r.ID == "" || r.CPUCores < 1 || !moFinite(r.CPUCapacity) || r.CPUCapacity <= 0 || r.MemoryBytes < 0 {
			return fmt.Errorf("resource %s has invalid CPU/memory capacity", r.ID)
		}
		if !moFinite(r.ComputeSpeedup) || r.ComputeSpeedup <= 0 || !moFinite(r.PricePerSecond) || r.PricePerSecond < 0 {
			return fmt.Errorf("resource %q requires positive computeSpeedup and nonnegative pricePerSecond", r.ID)
		}
		if r.BootOverheadSeconds < 0 || !moFinite(r.BootOverheadSeconds) || r.ContainerOverheadSeconds < 0 || !moFinite(r.ContainerOverheadSeconds) {
			return fmt.Errorf("invalid overhead for %s", r.ID)
		}
		for _, key := range []string{"storagePricePerByteSecond", "ingressPricePerByte", "egressPricePerByte"} {
			if raw, ok := r.Metadata[key]; ok {
				number, valid := raw.(float64)
				if !valid || !moFinite(number) || number < 0 {
					return fmt.Errorf("invalid price %s on %s", key, r.ID)
				}
			}
		}
		m.resources = append(m.resources, r)
	}
	sort.Slice(m.resources, func(i, j int) bool { return m.resources[i].ID < m.resources[j].ID })
	if len(m.resources) == 0 {
		return fmt.Errorf("scope has no schedulable resources")
	}

	return nil
}

func (m *moModel) loadGraph() error {
	index := map[string]int{}
	m.index = index
	for i, a := range m.tasks {
		if a.ID == "" {
			return fmt.Errorf("activity id is required")
		}
		if _, ok := index[a.ID]; ok {
			return fmt.Errorf("duplicate activity %q", a.ID)
		}
		if a.Resources.MemoryBytes < 0 || a.Resources.StorageBytes < 0 {
			return fmt.Errorf("negative resource requirement on %s", a.ID)
		}
		index[a.ID] = i
		if !moFinite(a.Resources.CPU) || a.Resources.CPU < 0 || a.Resources.CPU > 1 {
			return fmt.Errorf("MOHEFT plugin requires single-lane activities; %q requests %g CPUs", a.ID, a.Resources.CPU)
		}
	}
	var topology struct {
		ID    string
		Links []moLink
	}
	if len(m.input.Request.NetworkTopology) > 0 {
		if err := json.Unmarshal(m.input.Request.NetworkTopology, &topology); err != nil {
			return fmt.Errorf("network topology: %w", err)
		}
	}
	m.topologyID = topology.ID
	for _, l := range topology.Links {
		if !moFinite(l.BandwidthBitsPerSecond) || l.BandwidthBitsPerSecond <= 0 || !moFinite(l.LatencySeconds) || l.LatencySeconds < 0 || !moFinite(l.PricePerByte) || l.PricePerByte < 0 {
			return fmt.Errorf("invalid network link %s -> %s", l.SourceResourceID, l.TargetResourceID)
		}
		m.links[[2]string{l.SourceResourceID, l.TargetResourceID}] = l
		if l.Bidirectional {
			m.links[[2]string{l.TargetResourceID, l.SourceResourceID}] = l
		}
	}

	return m.loadDependencies()
}

func (m *moModel) loadDependencies() error {
	edges := map[[2]int]int64{}
	for _, d := range m.input.Request.Workflow.Dependencies {
		child, cok := m.index[d.ActivityID]
		parent, pok := m.index[d.DependsOnActivityID]
		if !cok || !pok || parent == child {
			return fmt.Errorf("invalid workflow dependency")
		}
		edges[[2]int{parent, child}] = 0
	}
	for _, raw := range m.input.Request.Workflow.DataDependencies {
		var d moData
		if err := json.Unmarshal(raw, &d); err != nil {
			return err
		}
		parent, pok := m.index[d.ProducerActivityID]
		child, cok := m.index[d.ConsumerActivityID]
		if !pok || !cok || parent == child || d.SizeBytes < 0 {
			return fmt.Errorf("invalid data dependency")
		}
		key := [2]int{parent, child}
		if _, ok := edges[key]; !ok {
			return fmt.Errorf("data edge must have a matching precedence dependency")
		}
		if d.SizeBytes > math.MaxInt64-edges[key] {
			return fmt.Errorf("data size overflow")
		}
		edges[key] += d.SizeBytes
	}
	m.parents = make([][]moEdge, len(m.tasks))
	m.children = make([][]moEdge, len(m.tasks))
	for key, size := range edges {
		m.parents[key[1]] = append(m.parents[key[1]], moEdge{key[0], size})
		m.children[key[0]] = append(m.children[key[0]], moEdge{key[1], size})
	}
	for i := range m.tasks {
		sort.Slice(m.parents[i], func(a, b int) bool { return m.tasks[m.parents[i][a].task].ID < m.tasks[m.parents[i][b].task].ID })
		sort.Slice(m.children[i], func(a, b int) bool { return m.tasks[m.children[i][a].task].ID < m.tasks[m.children[i][b].task].ID })
	}

	return nil
}

func (m *moModel) loadCapacity() error {
	m.runtimes = make([][]float64, len(m.tasks))
	m.lanes = make([]int, len(m.resources))
	for r, resource := range m.resources {
		count := resource.CPUCores
		if count < 1 {
			count = 1
		}
		if resource.CPUCapacity > 0 && resource.CPUCapacity < float64(count) {
			count = max(1, int(math.Floor(resource.CPUCapacity)))
		}
		if count > 65536 {
			return fmt.Errorf("resource has too many CPU lanes")
		}
		m.lanes[r] = count
	}
	m.memoryCheck = make([]bool, len(m.resources))
	for r, resource := range m.resources {
		maxMemory := int64(0)
		for task, a := range m.tasks {
			if m.fits(task, r) && a.Resources.MemoryBytes > maxMemory {
				maxMemory = a.Resources.MemoryBytes
			}
		}
		m.memoryCheck[r] = maxMemory > resource.MemoryBytes/int64(m.lanes[r])
	}

	return nil
}

func (m *moModel) loadRuntimes() error {
	if err := m.loadCapacity(); err != nil {
		return err
	}
	profiles := []moProfile{}
	for _, raw := range m.input.Request.ActivityProfiles {
		var p moProfile
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		profiles = append(profiles, p)
	}
	for i, a := range m.tasks {
		base := 0.0
		hasEstimate := a.Simulation != nil
		if a.Simulation != nil {
			base = a.Simulation.DurationSeconds
		}
		if !moFinite(base) || base < 0 {
			return fmt.Errorf("invalid duration for %q", a.ID)
		}
		m.runtimes[i] = make([]float64, len(m.resources))
		for j, r := range m.resources {
			hasEstimate := hasEstimate
			seconds := base / r.ComputeSpeedup
			for _, p := range profiles {
				if p.ActivityTypeID == a.ActivityTypeID && p.ResourceID == r.ID && p.RuntimeSeconds > 0 {
					hasEstimate = true
					seconds = p.RuntimeSeconds
					if p.ModelVersion == "reference-work-v1" {
						if a.Simulation == nil {
							return fmt.Errorf("reference-work profile requires declared work for %s", a.ID)
						}
						seconds *= base
					} else if p.ModelVersion == "compute-work-v1" {
						work, known := a.Metadata["computeWorkUnits"].(float64)
						if !known || !moFinite(work) || work < 0 {
							return fmt.Errorf("compute-work profile requires declared work for %s", a.ID)
						}
						seconds *= work
					}
					break
				}
			}
			if !hasEstimate || !moFinite(seconds) || seconds < 0 {
				return fmt.Errorf("invalid runtime for %s on %s", a.ID, r.ID)
			}
			m.runtimes[i][j] = seconds
		}
	}

	return nil
}

func (m *moModel) selectLane(s moState, task, resource int, ready, runtime float64) (int, float64) {
	r := m.resources[resource]
	lane, start := 0, math.Inf(1)
	for i, slots := range s.slots[resource] {
		available := math.Max(ready, r.BootOverheadSeconds)
		for _, slot := range slots {
			if available+r.ContainerOverheadSeconds+runtime <= slot.start {
				break
			}
			if available < slot.finish {
				available = slot.finish
			}
		}
		for m.memoryCheck[resource] {
			retry := moMemoryReady(s.memory[resource], available, available+r.ContainerOverheadSeconds+runtime, m.tasks[task].Resources.MemoryBytes, r.MemoryBytes)
			if retry <= available {
				break
			}
			available = retry
			for _, slot := range slots {
				if available+r.ContainerOverheadSeconds+runtime <= slot.start {
					break
				}
				if available < slot.finish {
					available = slot.finish
				}
			}
		}
		candidate := available + r.ContainerOverheadSeconds
		if candidate < start {
			lane, start = i, candidate
		}
	}

	return lane, start
}
