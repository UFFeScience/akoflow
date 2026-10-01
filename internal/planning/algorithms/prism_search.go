package algorithms

import (
	"container/heap"
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func runCompactPRISMSearch(
	ctx context.Context,
	search compactPRISMContext,
	objective string,
	progress ports.ProgressReporter,
) ([]compactPRISMState, error) {
	states := []compactPRISMState{initialCompactPRISMState(search)}
	pruneSLA := search.request.DeadlineSeconds > 0 || search.request.Budget > 0
	expander := newCompactPRISMExpander(ctx, search)
	defer expander.close()
	for step := 0; step < len(search.activities); step++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		expanded := expander.expand(states, step)
		if len(expanded) == 0 {
			return nil, fmt.Errorf("no feasible PRISM state at step %d", step+1)
		}
		if pruneSLA {
			withinSLA := pruneCompactPRISMStates(expanded, search.request)
			if len(withinSLA) > 0 {
				expanded = withinSLA
			} else {
				// Monotonic limits cannot become feasible again. Continue from the
				// best violating frontier so the caller still receives complete,
				// explicitly infeasible best-effort plans.
				pruneSLA = false
			}
		}
		states = selectCompactPRISMBeam(expanded, search.beamWidth, objective)
		if progress != nil {
			_ = progress.Report(
				ctx,
				float64(step+1)/float64(len(search.activities)),
				fmt.Sprintf("step-%d", step+1),
			)
		}
	}
	return states, nil
}

type compactPRISMExpansionJob struct {
	state       compactPRISMState
	step        int
	resultIndex int
	results     [][]compactPRISMState
	done        *sync.WaitGroup
}

type compactPRISMExpander struct {
	ctx     context.Context
	search  compactPRISMContext
	jobs    chan compactPRISMExpansionJob
	workers sync.WaitGroup
}

func newCompactPRISMExpander(
	ctx context.Context,
	search compactPRISMContext,
) *compactPRISMExpander {
	expander := &compactPRISMExpander{
		ctx: ctx, search: search,
		jobs: make(chan compactPRISMExpansionJob, runtime.GOMAXPROCS(0)),
	}
	workerCount := max(1, runtime.GOMAXPROCS(0))
	expander.workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer expander.workers.Done()
			for job := range expander.jobs {
				if expander.ctx.Err() == nil {
					job.results[job.resultIndex] = expandCompactPRISMState(
						expander.search,
						job.state,
						job.step,
					)
				}
				job.done.Done()
			}
		}()
	}
	return expander
}

func (expander *compactPRISMExpander) close() {
	close(expander.jobs)
	expander.workers.Wait()
}

func (expander *compactPRISMExpander) expand(
	states []compactPRISMState,
	step int,
) []compactPRISMState {
	results := make([][]compactPRISMState, len(states))
	var done sync.WaitGroup
	done.Add(len(states))
	for index, state := range states {
		expander.jobs <- compactPRISMExpansionJob{
			state: state, step: step, resultIndex: index,
			results: results, done: &done,
		}
	}
	done.Wait()
	total := 0
	for _, children := range results {
		total += len(children)
	}
	expanded := make([]compactPRISMState, 0, total)
	for _, children := range results {
		expanded = append(expanded, children...)
	}
	return expanded
}

func pruneCompactPRISMStates(
	states []compactPRISMState,
	request domain.PlanningRequest,
) []compactPRISMState {
	if request.DeadlineSeconds <= 0 && request.Budget <= 0 {
		return states
	}
	kept := states[:0]
	for _, state := range states {
		if compactPRISMWithinConstraints(state, request) {
			kept = append(kept, state)
		}
	}
	return kept
}

func compactPRISMWithinConstraints(
	state compactPRISMState,
	request domain.PlanningRequest,
) bool {
	return (request.DeadlineSeconds <= 0 || state.makespan <= request.DeadlineSeconds) &&
		(request.Budget <= 0 || state.cost <= request.Budget)
}

type compactPRISMStateHeap struct {
	states    []compactPRISMState
	objective string
}

func (items compactPRISMStateHeap) Len() int { return len(items.states) }
func (items compactPRISMStateHeap) Less(i, j int) bool {
	// Keep the worst selected item at the root so it can be replaced by a
	// better state in O(log k).
	return compactPRISMStateLess(items.states[j], items.states[i], items.objective)
}
func (items compactPRISMStateHeap) Swap(i, j int) {
	items.states[i], items.states[j] = items.states[j], items.states[i]
}
func (items *compactPRISMStateHeap) Push(value any) {
	items.states = append(items.states, value.(compactPRISMState))
}
func (items *compactPRISMStateHeap) Pop() any {
	old := items.states
	value := old[len(old)-1]
	items.states = old[:len(old)-1]
	return value
}

func compactPRISMTopStates(
	states []compactPRISMState,
	limit int,
	objective string,
) []compactPRISMState {
	if limit >= len(states) {
		selected := append([]compactPRISMState(nil), states...)
		sort.Slice(selected, func(i, j int) bool {
			return compactPRISMStateLess(selected[i], selected[j], objective)
		})
		return selected
	}
	selected := &compactPRISMStateHeap{
		states: make([]compactPRISMState, 0, limit), objective: objective,
	}
	for _, state := range states {
		if selected.Len() < limit {
			heap.Push(selected, state)
			continue
		}
		if compactPRISMStateLess(state, selected.states[0], objective) {
			selected.states[0] = state
			heap.Fix(selected, 0)
		}
	}
	sort.Slice(selected.states, func(i, j int) bool {
		return compactPRISMStateLess(selected.states[i], selected.states[j], objective)
	})
	return selected.states
}

func expandCompactPRISMState(
	search compactPRISMContext,
	state compactPRISMState,
	step int,
) []compactPRISMState {
	ready := compactPRISMReadyOrdinals(search, state)
	children := make(
		[]compactPRISMState,
		0,
		len(ready)*len(search.resources),
	)
	for _, activityOrdinal := range ready {
		prepared := compactPRISMPrepareFanIn(search, state, activityOrdinal)
		for resourceOrdinal := range search.resources {
			if !search.feasible[activityOrdinal][resourceOrdinal] {
				continue
			}
			children = append(children, compactPRISMPlacePreparedAlternatives(
				search,
				state,
				activityOrdinal,
				resourceOrdinal,
				step,
				true,
				prepared,
			)...)
		}
	}
	return children
}

func selectCompactPRISMBeam(
	states []compactPRISMState,
	width int,
	objective string,
) []compactPRISMState {
	states = dedupeCompactPRISMStates(states, objective)
	if len(states) <= width {
		return states
	}
	// Preserve explicit search diversity. Most of the beam follows the requested
	// objective, while dedicated lanes retain concrete partial schedules and
	// data-local placements that would otherwise disappear on early heuristic ties.
	primaryWidth := max(1, width*7/10)
	localityWidth := max(1, width*10/100)
	diversityWidth := 0
	for _, state := range states {
		if len(state.resourceAssignments) > 1 {
			diversityWidth = max(1, width*20/100)
			break
		}
	}
	if diversityWidth > 0 {
		// Interference avoidance commonly requires a sequence of placements that
		// looks locally slower before the extra resource capacity pays off. Keep a
		// dedicated lane of resource-balanced states so a bounded beam does not
		// collapse to one fast (or free) machine before that benefit is visible.
		diversityWidth = max(1, width*20/100)
	}
	secondaryWidth := width - primaryWidth - localityWidth - diversityWidth
	if secondaryWidth < 1 {
		secondaryWidth = 1
		primaryWidth = max(1, width-localityWidth-diversityWidth-secondaryWidth)
	}
	primary := compactPRISMTopStates(states, primaryWidth, objective)
	secondaryObjective := "cost"
	if objective == "cost" {
		secondaryObjective = "time"
	} else {
		// Preserve a quarter of the time beam by the concrete partial schedule,
		// rather than by cost. This keeps low-wait/core-balanced alternatives
		// alive when the projected critical-path heuristic ties.
		secondaryObjective = "earliest-finish"
	}
	secondary := compactPRISMTopStates(states, width, secondaryObjective)
	locality := compactPRISMTopStates(states, width, "network-locality")
	diversity := compactPRISMTopStates(states, width, "resource-diversity")
	selected := append([]compactPRISMState(nil), primary...)
	seen := make(map[uint64]bool, width)
	for _, state := range selected {
		seen[state.signature] = true
	}
	selected = appendCompactPRISMLane(selected, seen, secondary, secondaryWidth, width)
	selected = appendCompactPRISMLane(selected, seen, locality, localityWidth, width)
	selected = appendCompactPRISMLane(selected, seen, diversity, diversityWidth, width)
	// Overlap between lanes can leave spare capacity. Fill it by the primary
	// objective so the effective beam width remains stable.
	for _, state := range compactPRISMTopStates(states, width, objective) {
		if seen[state.signature] {
			continue
		}
		selected = append(selected, state)
		seen[state.signature] = true
		if len(selected) == width {
			break
		}
	}
	return selected
}

func appendCompactPRISMLane(selected []compactPRISMState, seen map[uint64]bool, lane []compactPRISMState, limit, width int) []compactPRISMState {
	for _, state := range lane {
		if limit <= 0 || len(selected) >= width {
			return selected
		}
		if seen[state.signature] {
			continue
		}
		selected = append(selected, state)
		seen[state.signature] = true
		limit--
	}
	return selected
}

func compactPRISMStateLess(
	left compactPRISMState,
	right compactPRISMState,
	objective string,
) bool {
	priorities := compactPRISMPriorities(left, right, objective)
	for _, priority := range priorities {
		if priority != 0 {
			return priority < 0
		}
	}
	return left.signature < right.signature
}

func compactPRISMPriorities(left, right compactPRISMState, objective string) []int {
	leftMax, rightMax := compactPRISMMaxAssignments(left), compactPRISMMaxAssignments(right)
	ascending := func(a, b uint64) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}
	priority := func(a, b float64) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}
	if objective == "network-locality" {
		return []int{
			priority(left.transferSeconds, right.transferSeconds), priority(left.networkCost, right.networkCost),
			ascending(uint64(left.usedResourceCount), uint64(right.usedResourceCount)),
			priority(left.projectedMakespan, right.projectedMakespan), priority(left.projectedCost, right.projectedCost),
		}
	}
	if objective == "earliest-finish" {
		return []int{
			priority(left.makespan, right.makespan), priority(left.queueSeconds, right.queueSeconds),
			priority(left.projectedMakespan, right.projectedMakespan), priority(left.projectedCost, right.projectedCost),
		}
	}
	if objective == "resource-diversity" {
		return []int{
			ascending(uint64(right.usedResourceCount), uint64(left.usedResourceCount)), ascending(uint64(leftMax), uint64(rightMax)),
			priority(left.projectedMakespan, right.projectedMakespan), priority(left.transferSeconds, right.transferSeconds),
		}
	}
	if objective == "cost" {
		return []int{
			priority(left.projectedCost, right.projectedCost), priority(left.networkCost, right.networkCost), priority(left.transferSeconds, right.transferSeconds),
			ascending(uint64(left.usedResourceCount), uint64(right.usedResourceCount)), priority(left.projectedMakespan, right.projectedMakespan), priority(left.queueSeconds, right.queueSeconds),
		}
	}
	return []int{
		priority(left.projectedMakespan, right.projectedMakespan), priority(left.transferSeconds, right.transferSeconds), priority(left.networkCost, right.networkCost),
		ascending(uint64(left.usedResourceCount), uint64(right.usedResourceCount)), priority(left.queueSeconds, right.queueSeconds), priority(left.projectedCost, right.projectedCost),
	}
}

func compactPRISMMaxAssignments(state compactPRISMState) uint32 {
	var maxValue uint32
	for _, count := range state.resourceAssignments {
		maxValue = max(maxValue, count)
	}
	return maxValue
}

func dedupeCompactPRISMStates(
	states []compactPRISMState,
	objective string,
) []compactPRISMState {
	bySignature := make(map[uint64]compactPRISMState, len(states))
	for _, state := range states {
		previous, exists := bySignature[state.signature]
		if !exists || compactPRISMCompleteLess(state, previous, objective) {
			bySignature[state.signature] = state
		}
	}
	unique := make([]compactPRISMState, 0, len(bySignature))
	for _, state := range bySignature {
		unique = append(unique, state)
	}
	return unique
}

func compactPRISMCompleteLess(
	left compactPRISMState,
	right compactPRISMState,
	objective string,
) bool {
	if objective == "cost" {
		if left.cost != right.cost {
			return left.cost < right.cost
		}
		if left.networkCost != right.networkCost {
			return left.networkCost < right.networkCost
		}
		if left.transferSeconds != right.transferSeconds {
			return left.transferSeconds < right.transferSeconds
		}
		if left.usedResourceCount != right.usedResourceCount {
			return left.usedResourceCount < right.usedResourceCount
		}
		return left.makespan < right.makespan
	}
	if left.makespan != right.makespan {
		return left.makespan < right.makespan
	}
	if left.transferSeconds != right.transferSeconds {
		return left.transferSeconds < right.transferSeconds
	}
	if left.networkCost != right.networkCost {
		return left.networkCost < right.networkCost
	}
	if left.usedResourceCount != right.usedResourceCount {
		return left.usedResourceCount < right.usedResourceCount
	}
	return left.cost < right.cost
}
