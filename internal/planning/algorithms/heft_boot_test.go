package algorithms

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestHEFTEmitsOnlyTheBootItPlanned(t *testing.T) {
	state := initialState()
	resource := domain.Resource{ID: "cpu", BootOverheadSeconds: 12, ContainerOverhead: 2}
	for i := 0; i < 2; i++ {
		placement := evaluateHEFTPlacement(state, resource, "cpu-core-1", 0, 0, 3)
		commitHEFTPlacement(&state, domain.Activity{ID: string(rune('a' + i))}, i, placement)
		assignment := state.assignments[i]
		if assignment.Metadata["bootOverheadSeconds"] != 0.0 || assignment.Metadata["resourceAvailableAtSeconds"] != 12.0 || assignment.Metadata["containerOverheadSeconds"] != float64(2) {
			t.Fatal(assignment.Metadata)
		}
	}
	if state.assignments[1].PredictedFinishAt != 22 {
		t.Fatal(state.assignments)
	}
}
