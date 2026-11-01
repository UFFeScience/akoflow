package main

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestCandidateSelectionUsesNativeForecastBeforeReplay(t *testing.T) {
	plans := []domain.SchedulePlan{{ID: "fast", Predicted: domain.PredictedMetrics{MakespanSeconds: 1, Cost: 10}}, {ID: "affordable", Predicted: domain.PredictedMetrics{MakespanSeconds: 3, Cost: 2}}, {ID: "cheap-late", Predicted: domain.PredictedMetrics{MakespanSeconds: 10, Cost: 0}}}
	p, n, best := selectPlan(plans, "time", 5, 3, true)
	if p.ID != "affordable" || n != 1 || best {
		t.Fatal("feasible-frontier time policy changed")
	}
	p, n, best = selectPlan(plans, "time", 5, 3, false)
	if p.ID != "fast" || n != 1 || best {
		t.Fatal("SLA-off must retain the unconstrained endpoint")
	}
	p, n, best = selectPlan(plans, "cost", 0.1, 0.1, true)
	if p.ID != "cheap-late" || n != 0 || !best {
		t.Fatal("no feasible prediction must remain explicit best effort")
	}
}
func TestSeparatePanelsDoNotCountSLAOffAsAnotherEnvironment(t *testing.T) {
	rows := []result{}
	for _, s := range scenarios {
		for _, panel := range []string{"time", "cost"} {
			for _, margin := range margins {
				for _, algorithm := range []string{"heft", "moheft-" + panel, "prism-" + panel} {
					r := result{ID: caseKey(s.ID, panel, margin, algorithm, algorithm == "prism-"+panel), Environment: s.ID, Panel: panel, Margin: margin, Algorithm: algorithm, SLAAware: algorithm == "prism-"+panel, ScheduleSHA: "on", SLAMet: true}
					if algorithm == "prism-"+panel {
						r.SLAMet = s.ID == "S1" || s.ID == "S2" || s.ID == "S3"
					}
					rows = append(rows, r)
				}
				rows = append(rows, result{ID: caseKey(s.ID, panel, margin, "prism-"+panel, false), Environment: s.ID, Panel: panel, Margin: margin, Algorithm: "prism-" + panel, ScheduleSHA: "off"})
			}
		}
	}
	summary, paired := summarize(rows)
	if len(summary) != 30 || len(paired) != 10 {
		t.Fatal("incorrect plot or paired-grid coverage")
	}
	for _, s := range summary {
		if s.N != 7 {
			t.Fatal("ablation inflated the main denominator")
		}
		if s.Algorithm == "prism-"+s.Panel && s.Met != 3 {
			t.Fatal("wrong main PRISM compliance")
		}
	}
	for _, p := range paired {
		if p.N != 7 || p.OnMet != 3 || p.OffMet != 0 || p.OnOnly != 3 || p.Neither != 4 || p.DifferentSchedules != 7 {
			t.Fatal("paired ablation counting error", p)
		}
	}
}
func TestNativeAuditRejectsWrongAssignmentsAndBrokenLaneOrder(t *testing.T) {
	workflow := domain.WorkflowVersion{Activities: []domain.Activity{{ID: "a"}, {ID: "b"}}, Dependencies: []domain.ActivityDependency{{ActivityID: "b", DependsOnActivityID: "a"}}}
	plan := domain.SchedulePlan{ID: "p", Assignments: []domain.PlanAssignment{{ID: "pa", PlanID: "p", ActivityID: "a", ResourceID: "cpu", CoreID: "lane", OrderOnResource: 0}, {ID: "pb", PlanID: "p", ActivityID: "b", ResourceID: "cpu", CoreID: "lane", OrderOnResource: 1}}}
	trace := domain.ExecutionTrace{PlanID: "p", Tasks: []domain.TaskExecution{{ActivityID: "a", PlanAssignmentID: "pa", Status: domain.TaskCompleted, PlannedResourceID: "cpu", AllocatedResourceID: "cpu", StartedAt: 0, FinishedAt: 1}, {ActivityID: "b", PlanAssignmentID: "pb", Status: domain.TaskCompleted, PlannedResourceID: "cpu", AllocatedResourceID: "cpu", StartedAt: 1, FinishedAt: 2}}}
	if n := auditTrace(plan, trace, workflow); n != 1 {
		t.Fatal("same DAG/lane relation counted twice")
	}
	for _, kind := range []string{"resource", "order", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			broken := trace
			broken.Tasks = append([]domain.TaskExecution(nil), trace.Tasks...)
			switch kind {
			case "resource":
				broken.Tasks[1].AllocatedResourceID = "other"
			case "order":
				broken.Tasks[1].StartedAt = 0.5
			case "duplicate":
				broken.Tasks[1] = broken.Tasks[0]
			}
			defer func() {
				if recover() == nil {
					t.Fatal("invalid native evidence passed audit")
				}
			}()
			auditTrace(plan, broken, workflow)
		})
	}
}
func TestSchedulerConfigurationIsNotMisreportedAsPRISMDefaults(t *testing.T) {
	c := &campaign{config: config{Beam: 20}}
	if len(c.schedulerConfiguration("heft", false)) != 0 {
		t.Fatal("HEFT received invented configuration")
	}
	mo := c.schedulerConfiguration("moheft-cost", false)
	if len(mo) != 1 || mo["candidateCount"] != 25 {
		t.Fatal("MOHEFT K was not preserved")
	}
	on, off := c.schedulerConfiguration("prism-time", true), c.schedulerConfiguration("prism-time", false)
	if on["slaAware"] != true || off["slaAware"] != false {
		t.Fatal("ablation toggle missing")
	}
	delete(on, "slaAware")
	delete(off, "slaAware")
	if !equalJSON(on, off) {
		t.Fatal("other search settings changed")
	}
}
