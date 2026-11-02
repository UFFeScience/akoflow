package algorithms

import (
	"context"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
	"testing"
)

// Independent reference: Topcuoglu et al. (2002), Figs. 3 and 4(a), pp. 265.
// doi:10.1109/71.993206. The published schedule length is exactly 80.
func TestHEFTReproducesPublishedTenTaskSchedule(t *testing.T) {
	times := [][3]float64{{14, 16, 9}, {13, 19, 18}, {11, 13, 19}, {13, 8, 17}, {12, 13, 10}, {13, 16, 9}, {7, 15, 11}, {5, 11, 14}, {18, 12, 20}, {21, 7, 16}}
	edges := [][3]int{{1, 2, 18}, {1, 3, 12}, {1, 4, 9}, {1, 5, 11}, {1, 6, 14}, {2, 8, 19}, {2, 9, 16}, {3, 7, 23}, {4, 8, 27}, {4, 9, 23}, {5, 9, 13}, {6, 8, 15}, {7, 10, 17}, {8, 10, 11}, {9, 10, 13}}
	id := func(n int) string { return fmt.Sprintf("n%02d", n) }
	r := domain.PlanningRequest{Workflow: domain.WorkflowVersion{ID: "paper"}, ExecutionScope: domain.ExecutionScope{ID: "scope", EnvironmentVersionIDs: []string{"env"}}}
	for p := 1; p <= 3; p++ {
		r.Resources = append(r.Resources, domain.Resource{ID: fmt.Sprintf("p%d", p), EnvironmentVersionID: "env", CPUCores: 1, CPUCapacity: 1, MemoryBytes: 1024, Schedulable: true})
	}
	for i, values := range times {
		r.Workflow.Activities = append(r.Workflow.Activities, domain.Activity{ID: id(i + 1), ActivityTypeID: id(i + 1), Resources: domain.ActivityResources{CPU: 1}})
		for p, v := range values {
			r.ActivityProfiles = append(r.ActivityProfiles, domain.ActivityResourceProfile{ActivityTypeID: id(i + 1), ResourceID: fmt.Sprintf("p%d", p+1), RuntimeSeconds: v})
		}
	}
	for _, e := range edges {
		r.Workflow.Dependencies = append(r.Workflow.Dependencies, domain.ActivityDependency{ActivityID: id(e[1]), DependsOnActivityID: id(e[0])})
		r.Workflow.DataDependencies = append(r.Workflow.DataDependencies, domain.ActivityDataDependency{ProducerActivityID: id(e[0]), ConsumerActivityID: id(e[1]), SizeBytes: int64(e[2])})
	}
	for p := 1; p <= 3; p++ {
		for q := p + 1; q <= 3; q++ {
			r.NetworkTopology.Links = append(r.NetworkTopology.Links, domain.NetworkLink{SourceResourceID: fmt.Sprintf("p%d", p), TargetResourceID: fmt.Sprintf("p%d", q), BandwidthBitsPerSecond: 8, Bidirectional: true})
		}
	}
	sink := &testSink{}
	require.NoError(t, (HEFT{}).Schedule(context.Background(), r, nil, nil, sink))
	plan := sink.plans[0]
	require.Equal(t, 80.0, plan.Predicted.MakespanSeconds)
	order := []string{}
	for _, a := range plan.Assignments {
		order = append(order, a.ActivityID)
	}
	// n3 and n4 both have rank 80. The paper permits random tie-breaking;
	// either order is valid and yields the same published processor schedule.
	require.Equal(t, "n01", order[0])
	require.ElementsMatch(t, []string{"n03", "n04"}, order[1:3])
	require.Equal(t, []string{"n02", "n05", "n06", "n09", "n07", "n08", "n10"}, order[3:])
	expected := []struct {
		processor     string
		start, finish float64
	}{{"p3", 0, 9}, {"p1", 27, 40}, {"p3", 9, 28}, {"p2", 18, 26}, {"p3", 28, 38}, {"p2", 26, 42}, {"p3", 38, 49}, {"p1", 57, 62}, {"p2", 56, 68}, {"p2", 73, 80}}
	byID := map[string]domain.PlanAssignment{}
	for _, a := range plan.Assignments {
		byID[a.ActivityID] = a
	}
	for i, w := range expected {
		a := byID[id(i+1)]
		require.Equal(t, w.processor, a.ResourceID, id(i+1))
		require.Equal(t, w.start, a.PredictedStartAt, id(i+1))
		require.Equal(t, w.finish, a.PredictedFinishAt, id(i+1))
	}
}
func TestHEFTInsertsIntoGapAndDoesNotChooseByGlobalMakespan(t *testing.T) {
	s := initialState()
	s.assignments = []domain.PlanAssignment{{CoreID: "core", PredictedStartAt: 0, PredictedFinishAt: 5}, {CoreID: "core", PredictedStartAt: 20, PredictedFinishAt: 30}}
	s.coreFree["core"] = 30
	s.makespan = 100
	got := evaluateHEFTPlacement(s, domain.Resource{}, "core", 6, 0, 5)
	require.Equal(t, 6., got.start)
	require.Equal(t, 11., got.finish)
	selected := heftPlacement{finish: 20, makespan: 100, cost: 0}
	require.True(t, betterHEFTPlacement(heftPlacement{finish: 11, makespan: 100, cost: 1}, &selected))
}

func TestHEFTPersistsChronologicalOrderAfterGapInsertion(t *testing.T) {
	r := domain.PlanningRequest{Workflow: domain.WorkflowVersion{ID: "gap"}, ExecutionScope: domain.ExecutionScope{EnvironmentVersionIDs: []string{"env"}}}
	for _, id := range []string{"p1", "p2"} {
		r.Resources = append(r.Resources, domain.Resource{ID: id, EnvironmentVersionID: "env", CPUCores: 1, CPUCapacity: 1, MemoryBytes: 1024, Schedulable: true})
	}
	for _, row := range []struct {
		id   string
		a, b float64
	}{{"parent", 10, 1000}, {"late", 1000, 1}, {"tail", 1000, 1000}, {"gap", 1000, 5}} {
		r.Workflow.Activities = append(r.Workflow.Activities, domain.Activity{ID: row.id, ActivityTypeID: row.id, Resources: domain.ActivityResources{CPU: 1}})
		r.ActivityProfiles = append(r.ActivityProfiles, domain.ActivityResourceProfile{ActivityTypeID: row.id, ResourceID: "p1", RuntimeSeconds: row.a}, domain.ActivityResourceProfile{ActivityTypeID: row.id, ResourceID: "p2", RuntimeSeconds: row.b})
	}
	r.Workflow.Dependencies = []domain.ActivityDependency{{ActivityID: "late", DependsOnActivityID: "parent"}, {ActivityID: "tail", DependsOnActivityID: "late"}}
	sink := &testSink{}
	require.NoError(t, (HEFT{}).Schedule(context.Background(), r, nil, nil, sink))
	byID := map[string]domain.PlanAssignment{}
	for _, a := range sink.plans[0].Assignments {
		byID[a.ActivityID] = a
	}
	require.Equal(t, 0., byID["gap"].PredictedStartAt)
	require.Equal(t, 5., byID["gap"].PredictedFinishAt)
	require.Equal(t, 0, byID["gap"].OrderOnResource)
	require.Equal(t, 1, byID["late"].OrderOnResource)
}
