package execution

import (
	"reflect"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestPlanActivityTestGraph(t *testing.T) {
	workflow := domain.WorkflowVersion{
		Activities: []domain.Activity{
			{ID: "upstream"}, {ID: "cached"}, {ID: "other-input"}, {ID: "selected"}, {ID: "downstream"},
		},
		Dependencies: []domain.ActivityDependency{
			{ActivityID: "cached", DependsOnActivityID: "upstream"},
			{ActivityID: "selected", DependsOnActivityID: "cached"},
			{ActivityID: "downstream", DependsOnActivityID: "selected"},
		},
		DataDependencies: []domain.ActivityDataDependency{
			{ConsumerActivityID: "selected", ProducerActivityID: "other-input"},
		},
	}
	graph, err := PlanActivityTestGraph(workflow, "selected", map[string]bool{"cached": true, "selected": true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(graph.RunActivityIDs, []string{"other-input", "selected"}) ||
		!reflect.DeepEqual(graph.ReuseActivityIDs, []string{"cached"}) {
		t.Fatalf("unexpected test graph: %+v", graph)
	}
	workflow.Dependencies = append(workflow.Dependencies, domain.ActivityDependency{
		ActivityID: "upstream", DependsOnActivityID: "selected",
	})
	if _, err := PlanActivityTestGraph(workflow, "selected", nil); err == nil {
		t.Fatal("dependency cycle must prevent submission")
	}
}
