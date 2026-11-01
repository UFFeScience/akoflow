package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainplanning "github.com/UFFeScience/akoflow/internal/domain/planning"
	validator "github.com/UFFeScience/akoflow/internal/infrastructure/plugins/planning"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func within(value, limit float64) bool { return value <= limit+1e-9*math.Max(1, math.Abs(limit)) }
func clonePlan(p domain.SchedulePlan) domain.SchedulePlan {
	b, e := json.Marshal(p)
	must(e)
	var x domain.SchedulePlan
	must(json.Unmarshal(b, &x))
	return x
}
func renamePlan(p domain.SchedulePlan, id string) domain.SchedulePlan {
	p = clonePlan(p)
	old := p.ID
	mapping := map[string]string{}
	for i := range p.Assignments {
		a := &p.Assignments[i]
		mapping[a.ID] = id + "-" + a.ActivityID
		a.ID = mapping[a.ID]
		a.PlanID = id
	}
	for i := range p.LifecycleActions {
		a := &p.LifecycleActions[i]
		mapping[a.ID] = strings.Replace(a.ID, old, id, 1)
		a.ID = mapping[a.ID]
		a.SchedulePlanID = id
	}
	for i := range p.LifecycleActions {
		for j, d := range p.LifecycleActions[i].DependsOn {
			if replacement, ok := mapping[d]; ok {
				p.LifecycleActions[i].DependsOn[j] = replacement
			}
		}
	}
	p.ID = id
	return p
}
func scheduleHash(p domain.SchedulePlan) string {
	p = clonePlan(p)
	p = renamePlan(p, "structural-plan")
	p.DeadlineSeconds, p.Budget = 0, 0
	p.Algorithm, p.AlgorithmVersion, p.Objective, p.Source = "", "", "", ""
	p.Predicted = domain.PredictedMetrics{}
	p.Metadata = nil
	sort.Slice(p.Assignments, func(i, j int) bool { return p.Assignments[i].ActivityID < p.Assignments[j].ActivityID })
	sort.Slice(p.LifecycleActions, func(i, j int) bool { return p.LifecycleActions[i].ID < p.LifecycleActions[j].ID })
	b, e := json.Marshal(p)
	must(e)
	return digest(b)
}
func auditTrace(plan domain.SchedulePlan, trace domain.ExecutionTrace, workflow domain.WorkflowVersion) int {
	if trace.PlanID != plan.ID || len(trace.Tasks) != len(plan.Assignments) || len(trace.Tasks) != len(workflow.Activities) {
		panic("incomplete native trace")
	}
	bindings := map[string]domain.PlanAssignment{}
	tasks := map[string]domain.TaskExecution{}
	for _, a := range plan.Assignments {
		bindings[a.ActivityID] = a
	}
	for _, t := range trace.Tasks {
		a, ok := bindings[t.ActivityID]
		if !ok || tasks[t.ActivityID].ActivityID != "" || t.Status != domain.TaskCompleted || t.PlanAssignmentID != a.ID || t.PlannedResourceID != a.ResourceID || t.AllocatedResourceID != a.ResourceID || math.IsNaN(t.StartedAt) || math.IsInf(t.StartedAt, 0) || math.IsNaN(t.FinishedAt) || math.IsInf(t.FinishedAt, 0) || t.StartedAt < 0 || t.FinishedAt < t.StartedAt {
			panic("native assignment/task evidence mismatch " + t.ActivityID)
		}
		tasks[t.ActivityID] = t
	}
	relations := map[string]bool{}
	check := func(to, from string) {
		key := from + "/" + to
		if relations[key] {
			return
		}
		a, b := tasks[from], tasks[to]
		if a.ActivityID == "" || b.ActivityID == "" || b.StartedAt+1e-6 < a.FinishedAt {
			panic("native precedence violation " + key)
		}
		relations[key] = true
	}
	for _, d := range workflow.Dependencies {
		check(d.ActivityID, d.DependsOnActivityID)
	}
	for to, from := range domainplanning.LanePredecessors(plan.Assignments) {
		check(to, from)
	}
	return len(relations)
}
func (c *campaign) checkResult(r result) {
	if r.Completed != 6448 || r.ModelSHA != c.modelSHA || r.BinarySHA != c.binarySHA || r.InputSHA != c.inputSHA[r.Environment] || fileSHA(r.PlanPath) != r.PlanSHA || fileSHA(r.TracePath) != r.TraceSHA || fileSHA(r.NativePath) != r.NativeSHA {
		panic("completed case provenance changed " + r.ID)
	}
}
func (c *campaign) execute(ctx context.Context, s scenario, panel, algorithm string, aware bool, margin, d, b float64, plan domain.SchedulePlan, count, eligible int, bestEffort bool, planningSeconds float64) result {
	variant := ""
	if strings.HasPrefix(algorithm, "prism-") {
		if aware {
			variant = "-on"
		} else {
			variant = "-off"
		}
	}
	key := fmt.Sprintf("%s-%s-%03d-%s%s", s.ID, panel, int(math.Round(margin*100)), algorithm, variant)
	dir := filepath.Join(c.Root, "cases", key)
	id := "montage6448-sla-20261101-" + key
	plan = renamePlan(plan, id+"-plan")
	plan.DeadlineSeconds, plan.Budget = d, b
	plan.Predicted.Feasible = (d <= 0 || within(plan.Predicted.MakespanSeconds, d)) && (b <= 0 || within(plan.Predicted.Cost, b))
	r := c.inputs[s.ID]
	r.Plan = plan
	r.Run = domain.ExecutionRun{ID: id, Mode: domain.ExecutionModeSimulation, Seed: 911, SchedulePlanID: plan.ID}
	must(validator.NewValidator().Validate(plan, r.Workflow, r.Resources, r.ExecutionScope, r.NetworkTopology))
	save(filepath.Join(dir, "plan.json"), plan)
	save(filepath.Join(dir, "request-reference.json"), map[string]any{"frozen_input": filepath.Join(c.Root, "frozen", s.ID+".json"), "frozen_input_sha256": c.inputSHA[s.ID], "plan": "plan.json", "run": r.Run})
	started := time.Now()
	trace, e := c.executor.Execute(ctx, r)
	must(e)
	seconds := time.Since(started).Seconds()
	relations := auditTrace(plan, trace, r.Workflow)
	tracePath := filepath.Join(dir, "trace.json")
	save(tracePath, trace)
	nativeDirs, e := filepath.Glob(filepath.Join(c.Root, "native", id+"-simulation-*"))
	must(e)
	sort.Slice(nativeDirs, func(i, j int) bool {
		a, e := os.Stat(nativeDirs[i])
		must(e)
		b, e := os.Stat(nativeDirs[j])
		must(e)
		return a.ModTime().After(b.ModTime())
	})
	if len(nativeDirs) == 0 {
		panic("native bundle missing")
	}
	nativePath := filepath.Join(nativeDirs[0], "result.json")
	record := result{ID: key, Environment: s.ID, Panel: panel, Algorithm: algorithm, SLAAware: aware, Margin: margin, Deadline: d, Budget: b, Predicted: plan.Predicted, Simulated: trace.Executed, BestEffort: bestEffort, CandidateCount: count, EligibleCandidates: eligible, PlanID: plan.ID, RunID: id, PlanPath: filepath.Join(dir, "plan.json"), TracePath: tracePath, NativePath: nativePath, InputSHA: c.inputSHA[s.ID], ModelSHA: c.modelSHA, BinarySHA: c.binarySHA, ScheduleSHA: scheduleHash(plan), PlanningSeconds: planningSeconds, SimulationSeconds: seconds, Completed: len(trace.Tasks), VerifiedRelations: relations, CandidateOrigin: plan.Metadata["candidateOrigin"]}
	record.PlanSHA = fileSHA(record.PlanPath)
	record.TraceSHA = fileSHA(tracePath)
	record.NativeSHA = fileSHA(nativePath)
	record.DeadlineMet = d <= 0 || within(trace.Executed.MakespanSeconds, d)
	record.BudgetMet = b <= 0 || within(trace.Executed.Cost, b)
	record.SLAMet = record.DeadlineMet && record.BudgetMet
	record.PredictionSaysFeasible = plan.Predicted.Feasible
	return record
}
