package main

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
	"time"
)

func auditFixture() (domain.SchedulePlan, domain.WorkflowVersion, monitor) {
	plan := domain.SchedulePlan{ID: "p", Assignments: []domain.PlanAssignment{{ID: "p-a", ActivityID: "a", ResourceID: "r", CoreID: "c"}, {ID: "p-b", ActivityID: "b", ResourceID: "r", CoreID: "c", OrderOnResource: 1}}}
	trace := monitor{Run: domain.ExecutionRun{ID: "run", SchedulePlanID: "p", Status: domain.ExecutionRunCompleted}, Activities: []domain.TaskExecution{{PlanAssignmentID: "p-a", PlannedResourceID: "r", ActivityID: "a", AllocatedResourceID: "r", StartedAt: 1, Status: domain.TaskCompleted}, {PlanAssignmentID: "p-b", PlannedResourceID: "r", ActivityID: "b", AllocatedResourceID: "r", StartedAt: 3, Status: domain.TaskCompleted}}}
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

func TestAuditRejectsPhysicalNodeFallback(t *testing.T) {
	p, w, m := auditFixture()
	m.PhysicalNodes = map[string]string{"r": "diablo03"}
	for _, id := range []string{"a", "b"} {
		m.Handles = append(m.Handles, domain.ActivityHandle{ActivityID: id, Status: domain.HandleCompleted, Metadata: map[string]any{"plannedNode": "diablo03", "allocatedNode": "diablo03"}})
	}
	if r := audit(p, w, m); !r.Complete || r.PhysicalNodesVerified != 2 {
		t.Fatalf("matching physical nodes: %+v", r)
	}
	m.Handles[1].Metadata["allocatedNode"] = "diablo04"
	if r := audit(p, w, m); r.Complete || len(r.Violations) != 1 {
		t.Fatalf("fallback accepted: %+v", r)
	}
	m.Handles = m.Handles[:1]
	if r := audit(p, w, m); r.Complete || len(r.Violations) != 1 {
		t.Fatalf("missing node evidence accepted: %+v", r)
	}
}

func TestAuditRejectsMissingOrChangedAssignmentEvidence(t *testing.T) {
	for _, name := range []string{"assignment", "planned-resource", "allocation"} {
		t.Run(name, func(t *testing.T) {
			p, w, m := auditFixture()
			switch name {
			case "assignment":
				m.Activities[1].PlanAssignmentID = "another-plan-assignment"
			case "planned-resource":
				m.Activities[1].PlannedResourceID = ""
			case "allocation":
				m.Activities[1].AllocatedResourceID = ""
			}
			if r := audit(p, w, m); r.Complete || len(r.Violations) != 1 {
				t.Fatalf("%+v", r)
			}
		})
	}
}
