package algorithms

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"

	"github.com/UFFeScience/akoflow/internal/application/ports"
)

func runCompactPRISMSearch(
	ctx context.Context,
	search compactPRISMContext,
	objective string,
	progress ports.ProgressReporter,
) ([]compactPRISMState, error) {
	states := []compactPRISMState{initialCompactPRISMState(search)}
	for step := 0; step < len(search.activities); step++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		expanded := expandCompactPRISMStates(ctx, search, states, step)
		if len(expanded) == 0 {
			return nil, fmt.Errorf("no feasible PRISM state at step %d", step+1)
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

func expandCompactPRISMStates(
	ctx context.Context,
	search compactPRISMContext,
	states []compactPRISMState,
	step int,
) []compactPRISMState {
	workers := min(len(states), runtime.GOMAXPROCS(0))
	results := make([][]compactPRISMState, len(states))
	jobs := make(chan int)
	var group sync.WaitGroup
	group.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer group.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					continue
				}
				results[index] = expandCompactPRISMState(
					search,
					states[index],
					step,
				)
			}
		}()
	}
	for index := range states {
		jobs <- index
	}
	close(jobs)
	group.Wait()
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
		for resourceOrdinal := range search.resources {
			if !search.feasible[activityOrdinal][resourceOrdinal] {
				continue
			}
			children = append(children, compactPRISMPlace(
				search,
				state,
				activityOrdinal,
				resourceOrdinal,
				step,
				true,
			))
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
	primaryWidth := max(1, width*3/4)
	secondaryWidth := width - primaryWidth
	primary := append([]compactPRISMState(nil), states...)
	secondary := append([]compactPRISMState(nil), states...)
	sort.SliceStable(primary, func(i, j int) bool {
		return compactPRISMStateLess(primary[i], primary[j], objective)
	})
	secondaryObjective := "cost"
	if objective == "cost" {
		secondaryObjective = "time"
	}
	sort.SliceStable(secondary, func(i, j int) bool {
		return compactPRISMStateLess(secondary[i], secondary[j], secondaryObjective)
	})
	selected := append([]compactPRISMState(nil), primary[:primaryWidth]...)
	seen := make(map[uint64]bool, width)
	for _, state := range selected {
		seen[state.signature] = true
	}
	for _, state := range secondary {
		if seen[state.signature] {
			continue
		}
		selected = append(selected, state)
		seen[state.signature] = true
		if len(selected) == primaryWidth+secondaryWidth {
			break
		}
	}
	return selected
}

func compactPRISMStateLess(
	left compactPRISMState,
	right compactPRISMState,
	objective string,
) bool {
	if objective == "cost" {
		if left.projectedCost != right.projectedCost {
			return left.projectedCost < right.projectedCost
		}
		if left.projectedMakespan != right.projectedMakespan {
			return left.projectedMakespan < right.projectedMakespan
		}
	} else {
		if left.projectedMakespan != right.projectedMakespan {
			return left.projectedMakespan < right.projectedMakespan
		}
		if left.projectedCost != right.projectedCost {
			return left.projectedCost < right.projectedCost
		}
	}
	return left.signature < right.signature
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
		return left.makespan < right.makespan
	}
	if left.makespan != right.makespan {
		return left.makespan < right.makespan
	}
	return left.cost < right.cost
}
