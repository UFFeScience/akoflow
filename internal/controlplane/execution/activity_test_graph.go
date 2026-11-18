package execution

import (
	"fmt"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type ActivityTestGraph struct {
	RunActivityIDs   []string `json:"runActivityIds"`
	ReuseActivityIDs []string `json:"reuseActivityIds"`
}

// PlanActivityTestGraph selects the target's prerequisites. A reusable producer
// supplies its recorded workspace, so its own prerequisites need not run.
func PlanActivityTestGraph(workflow domain.WorkflowVersion, target string, reusable map[string]bool) (ActivityTestGraph, error) {
	known := make(map[string]bool, len(workflow.Activities))
	for _, activity := range workflow.Activities {
		known[activity.ID] = true
	}
	if !known[target] {
		return ActivityTestGraph{}, fmt.Errorf("activity %q is not in the original workflow", target)
	}

	const (
		visiting = iota + 1
		run
		reuse
	)
	disposition := make(map[string]int)
	var visit func(string) error
	visit = func(id string) error {
		switch disposition[id] {
		case visiting:
			return fmt.Errorf("dependency cycle at activity %q", id)
		case run, reuse:
			return nil
		}
		if !known[id] {
			return fmt.Errorf("required activity %q is not in the original workflow", id)
		}
		if id != target && reusable[id] {
			disposition[id] = reuse
			return nil
		}
		disposition[id] = visiting
		for _, producer := range workspaceProducers(workflow, id) {
			if err := visit(producer); err != nil {
				return err
			}
		}
		disposition[id] = run
		return nil
	}
	if err := visit(target); err != nil {
		return ActivityTestGraph{}, err
	}

	graph := ActivityTestGraph{RunActivityIDs: []string{}, ReuseActivityIDs: []string{}}
	for _, activity := range workflow.Activities {
		switch disposition[activity.ID] {
		case run:
			graph.RunActivityIDs = append(graph.RunActivityIDs, activity.ID)
		case reuse:
			graph.ReuseActivityIDs = append(graph.ReuseActivityIDs, activity.ID)
		}
	}
	return graph, nil
}
