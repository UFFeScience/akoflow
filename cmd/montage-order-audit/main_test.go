package main

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
	"time"
)

func auditFixture() (domain.SchedulePlan, domain.WorkflowVersion, monitor) {
	plan := domain.SchedulePlan{ID: "p", Assignments: []domain.PlanAssignment{{ActivityID: "a", ResourceID: "r", CoreID: "c"}, {ActivityID: "b", ResourceID: "r", CoreID: "c", OrderOnResource: 1}}}
	trace := monitor{Run: domain.ExecutionRun{ID: "run", SchedulePlanID: "p", Status: domain.ExecutionRunCompleted}, Activities: []domain.TaskExecution{{ActivityID: "a", AllocatedResourceID: "r", StartedAt: 1, Status: domain.TaskCompleted}, {ActivityID: "b", AllocatedResourceID: "r", StartedAt: 3, Status: domain.TaskCompleted}}}
	for _, v := range []struct {
		kind, id string
		second   int64
	}{{"activity.started", "a", 1}, {"activity.completed", "a", 2}, {"activity.started", "b", 3}, {"activity.completed", "b", 4}} {
		e := event{Type: v.kind, At: time.Unix(v.second, 0)}
		e.Payload.ActivityID = v.id
		trace.Events = append(trace.Events, e)
	}
	return plan, domain.WorkflowVersion{}, trace
}
func TestAuditRejectsLaneOvertake(t *testing.T) {
	p, w, m := auditFixture()
	m.Events[2].At = time.Unix(1, 500)
	r := audit(p, w, m)
	if len(r.Violations) != 1 || r.Complete {
		t.Fatalf("%+v", r)
	}
}
func TestAuditRequiresCompletedEvidence(t *testing.T) {
	p, w, m := auditFixture()
	m.Events = append(m.Events[:1], m.Events[2:]...)
	r := audit(p, w, m)
	if r.PendingRelations != 1 || r.Complete {
		t.Fatalf("%+v", r)
	}
}
func TestAuditChecksResourceAndPlan(t *testing.T) {
	p, w, m := auditFixture()
	m.Run.SchedulePlanID = "other"
	m.Activities[1].AllocatedResourceID = "wrong"
	r := audit(p, w, m)
	if len(r.Violations) != 2 || r.Complete {
		t.Fatalf("%+v", r)
	}
}
func TestAuditAllowsIndependentLanesAndChecksWorkflowDependencies(t *testing.T) {
	p, w, m := auditFixture()
	p.Assignments[1].CoreID = "other"
	m.Events[2].At = time.Unix(1, 500)
	if r := audit(p, w, m); !r.Complete {
		t.Fatalf("independent lanes: %+v", r)
	}
	w.Dependencies = []domain.ActivityDependency{{ActivityID: "b", DependsOnActivityID: "a"}}
	if r := audit(p, w, m); len(r.Violations) != 1 {
		t.Fatalf("dependency: %+v", r)
	}
}
func TestAuditAcceptsStrictLaneCompletion(t *testing.T) {
	p, w, m := auditFixture()
	r := audit(p, w, m)
	if !r.Complete || r.VerifiedRelations != 1 {
		t.Fatalf("%+v", r)
	}
}
