package execution

import (
	"fmt"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	domainplanning "github.com/UFFeScience/akoflow/internal/domain/planning"
)

// Reject inconsistent plans before creating a run or contacting infrastructure.
func validatePlannedPrecedence(request ports.ExecutionRequest) error {
	activities := indexActivities(request.Workflow.Activities)
	assigned := make(map[string]bool)
	for _, assignment := range request.Plan.Assignments {
		if _, exists := activities[assignment.ActivityID]; !exists || assigned[assignment.ActivityID] {
			return fmt.Errorf("plan must assign every activity exactly once: %q", assignment.ActivityID)
		}
		assigned[assignment.ActivityID] = true
	}
	previous := domainplanning.LanePredecessors(request.Plan.Assignments)
	state := make(map[string]int)
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("workflow dependencies conflict with planned lane order: cycle at %q", id)
		}
		if state[id] == 2 {
			return nil
		}
		if _, exists := activities[id]; !exists {
			return fmt.Errorf("unknown planned predecessor %q", id)
		}
		state[id] = 1
		predecessors := workspaceProducers(request.Workflow, id)
		if predecessor, exists := previous[id]; exists {
			predecessors = append(predecessors, predecessor)
		}
		for _, predecessor := range predecessors {
			if err := visit(predecessor); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for id := range activities {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}
