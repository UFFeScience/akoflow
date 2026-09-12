package workflow

import "testing"

func dynamicActivity(name string) Activity {
	return Activity{Name: name, Kind: ActivityKindTask, Capabilities: []ActivityCapability{ActivityCapabilityReal}, Command: ActivityCommand{Entrypoint: "true"}}
}

func TestMaterializeExpansionUsesDeterministicIDsWithoutMutatingVersion(t *testing.T) {
	base := WorkflowVersion{ID: "version-1", Activities: []Activity{{
		ID: "root", Name: "root", Kind: ActivityKindTask,
		Capabilities: []ActivityCapability{ActivityCapabilityReal}, Command: ActivityCommand{Entrypoint: "true"},
	}}}
	request := ExpansionRequest{
		ID: "request", WorkflowVersionID: base.ID, SourceActivityID: "root", SourceEventID: "event-1", Sequence: 1,
		Activities:   []ExpansionActivity{{Key: "child", Activity: dynamicActivity("child")}},
		Dependencies: []ExpansionDependency{{ActivityKey: "child", DependsOnActivityID: "root"}},
	}
	expansion, result, err := MaterializeExpansion(base, nil, request, ExpansionLimits{})
	if err != nil {
		t.Fatal(err)
	}
	wantID := DeterministicActivityID(base.ID, request.SourceEventID, "child")
	if expansion.Activities[0].ID != wantID || result.Revision != 1 {
		t.Fatalf("unexpected materialization: %+v", expansion)
	}
	if len(base.Activities) != 1 {
		t.Fatal("immutable workflow version was mutated")
	}
	again, _, err := MaterializeExpansion(base, nil, request, ExpansionLimits{})
	if err != nil || again.ID != expansion.ID || again.Activities[0].ID != wantID {
		t.Fatalf("replay changed deterministic output: %+v %v", again, err)
	}
}

func TestMaterializeExpansionRejectsCycleAndLimits(t *testing.T) {
	base := WorkflowVersion{ID: "version-1", Activities: []Activity{{
		ID: "root", Name: "root", Kind: ActivityKindTask,
		Capabilities: []ActivityCapability{ActivityCapabilityReal}, Command: ActivityCommand{Entrypoint: "true"},
	}}}
	request := ExpansionRequest{
		ID: "request", WorkflowVersionID: base.ID, SourceActivityID: "root", SourceEventID: "event-1", Sequence: 1,
		Activities: []ExpansionActivity{{Key: "a", Activity: dynamicActivity("a")}, {Key: "b", Activity: dynamicActivity("b")}},
		Dependencies: []ExpansionDependency{
			{ActivityKey: "a", DependsOnActivityKey: "b"},
			{ActivityKey: "b", DependsOnActivityKey: "a"},
		},
	}
	if _, _, err := MaterializeExpansion(base, nil, request, ExpansionLimits{}); err == nil {
		t.Fatal("cycle must be rejected")
	}
	request.Dependencies = nil
	if _, _, err := MaterializeExpansion(base, nil, request, ExpansionLimits{MaxActivities: 2, MaxDependencies: 10}); err == nil {
		t.Fatal("activity limit must be enforced")
	}
}
