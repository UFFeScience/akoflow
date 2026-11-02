package planning

import "testing"

func TestLanePredecessorsKeepIndependentCoresSlotsAndResources(t *testing.T) {
	assignments := []PlanAssignment{
		{ActivityID: "second", ResourceID: "r", CoreID: "c", OrderOnResource: 1},
		{ActivityID: "first", ResourceID: "r", CoreID: "c"},
		{ActivityID: "other-core", ResourceID: "r", CoreID: "d"},
		{ActivityID: "other-slot", ResourceID: "r", CoreID: "c", SlotID: "s"},
		{ActivityID: "other-resource", ResourceID: "other", CoreID: "c"},
	}
	previous := LanePredecessors(assignments)
	if len(previous) != 1 || previous["second"] != "first" {
		t.Fatalf("unexpected lane predecessors: %v", previous)
	}
	if assignments[0].ActivityID != "second" {
		t.Fatal("input plan mutated")
	}
}
