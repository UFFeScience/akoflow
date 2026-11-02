package algorithms

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// Gap insertion finds 32 2/3 s here, while the narrow PRISM beam alone finds
// 33 1/3 s. The baseline must survive independently of beam and option limits.
func prismHEFTRegressionRequest() domain.PlanningRequest {
	r := planningRequestFixture()
	r.Workflow.Activities = nil
	for i, seconds := range []float64{19, 24, 23, 9, 16, 12, 6, 2} {
		r.Workflow.Activities = append(r.Workflow.Activities, planningActivity(string(rune('a'+i)), seconds))
	}
	r.Workflow.Dependencies = []domain.ActivityDependency{
		{ActivityID: "b", DependsOnActivityID: "a"},
		{ActivityID: "f", DependsOnActivityID: "a"},
		{ActivityID: "g", DependsOnActivityID: "d"},
		{ActivityID: "g", DependsOnActivityID: "e"},
		{ActivityID: "h", DependsOnActivityID: "c"},
	}
	r.Resources[0].ContainerOverhead = 7
	r.Resources[1].ContainerOverhead = 2
	r.Resources[2].ComputeSpeedup = 1.5
	r.Resources[2].ContainerOverhead = 7
	return r
}

func TestPRISMTimeRetainsNativeHEFTOutsideNarrowBeam(t *testing.T) {
	r := prismHEFTRegressionRequest()
	configuration := map[string]any{"beamWidth": 1, "readyBranchLimit": 1, "optionCount": 1}
	search, err := newCompactPRISMContext(r, configuration)
	if err != nil {
		t.Fatal(err)
	}
	states, err := runCompactPRISMSearch(context.Background(), search, "time", nil)
	if err != nil {
		t.Fatal(err)
	}
	states, err = reevaluateCompleteCompactPRISMStates(search, states)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := compactPRISMHEFTAnchor(context.Background(), search)
	if err != nil {
		t.Fatal(err)
	}
	if states[0].makespan <= baseline.makespan+1e-8 {
		t.Fatal("fixture must expose loss of the better baseline in beam search")
	}
	sink := &testSink{}
	if err = NewPRISMTime().Schedule(context.Background(), r, configuration, nil, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.plans) != 1 {
		t.Fatalf("expected one candidate, got %d", len(sink.plans))
	}
	p := sink.plans[0]
	if math.Abs(p.Predicted.MakespanSeconds-98.0/3) > 1e-8 {
		t.Fatalf("lost HEFT reference: %v", p.Predicted)
	}
	if p.AlgorithmVersion != "4" || p.Metadata["candidateOrigin"] != "heft-baseline" || p.Metadata["heftBaselineIncluded"] != true {
		t.Fatalf("missing selection provenance: %+v", p)
	}
	native := &testSink{}
	if err = (HEFT{}).Schedule(context.Background(), r, nil, nil, native); err != nil {
		t.Fatal(err)
	}
	for i, a := range p.Assignments {
		b := native.plans[0].Assignments[i]
		if a.ActivityID != b.ActivityID || a.ResourceID != b.ResourceID || a.CoreID != b.CoreID || a.OrderOnResource != b.OrderOnResource || a.Priority != b.Priority {
			t.Fatalf("baseline placement changed: %+v != %+v", a, b)
		}
	}
}

func TestPRISMHEFTBaselineHonorsPlacementEligibilityAndCancellation(t *testing.T) {
	r := prismHEFTRegressionRequest()
	search, err := newCompactPRISMContext(r, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := range search.feasible {
		for j := range search.feasible[i] {
			search.feasible[i][j] = false
		}
	}
	if _, err = compactPRISMHEFTAnchor(context.Background(), search); !errors.Is(err, errPRISMHEFTBaselineIneligible) {
		t.Fatalf("ineligible baseline accepted: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = compactPRISMHEFTAnchor(ctx, search); !errors.Is(err, context.Canceled) {
		t.Fatalf("ignored cancellation: %v", err)
	}
}

func TestPRISMHEFTBaselineKeepsOriginalWorkflowBeforeWorkspaceProjection(t *testing.T) {
	_, request := mixedOriginPRISMFixture()
	search, err := newCompactPRISMContext(request, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(search.request.Workflow.DataDependencies, request.Workflow.DataDependencies) {
		t.Fatal("fixture must change edge volumes during workspace projection")
	}
	if !reflect.DeepEqual(search.heftRequest.Workflow.DataDependencies, request.Workflow.DataDependencies) {
		t.Fatal("HEFT reference received PRISM-projected edge volumes instead of the original input")
	}
}
