package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type aggregate struct {
	Panel           string  `json:"panel"`
	Algorithm       string  `json:"algorithm"`
	Margin          float64 `json:"margin"`
	N               int     `json:"n"`
	Met             int     `json:"met"`
	Percent         float64 `json:"percent"`
	DeadlineMet     int     `json:"deadline_met"`
	BudgetMet       int     `json:"budget_met"`
	PredictedMet    int     `json:"predicted_met"`
	FalseFeasible   int     `json:"false_feasible"`
	FalseInfeasible int     `json:"false_infeasible"`
	MonetaryN       int     `json:"monetary_n"`
	MonetaryMet     int     `json:"monetary_met"`
}
type ablation struct {
	Panel              string  `json:"panel"`
	Margin             float64 `json:"margin"`
	N                  int     `json:"n"`
	OnMet              int     `json:"on_met"`
	OffMet             int     `json:"off_met"`
	OnOnly             int     `json:"on_only"`
	OffOnly            int     `json:"off_only"`
	Both               int     `json:"both"`
	Neither            int     `json:"neither"`
	DifferentSchedules int     `json:"different_schedules"`
}

func equalJSON(a, b any) bool {
	x, e := json.Marshal(a)
	must(e)
	z, e := json.Marshal(b)
	must(e)
	return string(x) == string(z)
}
func caseKey(s, panel string, margin float64, algorithm string, aware bool) string {
	variant := ""
	if strings.HasPrefix(algorithm, "prism-") {
		if aware {
			variant = "-on"
		} else {
			variant = "-off"
		}
	}
	return fmt.Sprintf("%s-%s-%03d-%s%s", s, panel, int(math.Round(margin*100)), algorithm, variant)
}
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func (c *campaign) auditStored(r result, base result) int {
	c.checkResult(r)
	if !finite(r.Simulated.MakespanSeconds) || !finite(r.Simulated.Cost) || r.Simulated.MakespanSeconds <= 0 || r.Simulated.Cost < 0 {
		panic("invalid native metrics")
	}
	var p domain.SchedulePlan
	var trace, native domain.ExecutionTrace
	load(r.PlanPath, &p)
	expectedAlgorithm := r.Algorithm
	if strings.HasPrefix(expectedAlgorithm, "moheft-") {
		expectedAlgorithm = "moheft"
	}
	if p.Algorithm != expectedAlgorithm || p.AlgorithmVersion == "" {
		panic("selected plan algorithm/version differs from the scenario")
	}
	load(r.TracePath, &trace)
	load(r.NativePath, &native)
	if p.ID != r.PlanID || trace.RunID != r.RunID || trace.Mode != domain.ExecutionModeSimulation || trace.PlanID != p.ID || native.RunID != r.RunID || native.PlanID != p.ID || !within(math.Abs(native.Executed.MakespanSeconds-trace.Executed.MakespanSeconds), 1e-6) || !reflect.DeepEqual(trace.Executed, r.Simulated) || !reflect.DeepEqual(p.Predicted, r.Predicted) || scheduleHash(p) != r.ScheduleSHA {
		panic("stored native/processed/plan metrics disagree " + r.ID)
	}
	n := auditTrace(p, trace, c.inputs[r.Environment].Workflow)
	auditTrace(p, native, c.inputs[r.Environment].Workflow)
	if n != r.VerifiedRelations {
		panic("precedence audit count differs")
	}
	if r.Panel == "baseline" {
		return 0
	}
	if p.DeadlineSeconds != r.Deadline || p.Budget != r.Budget {
		panic("emitted limits erased")
	}
	if r.PredictionSaysFeasible != (within(r.Predicted.MakespanSeconds, r.Deadline) && within(r.Predicted.Cost, r.Budget)) || r.SLAMet != (within(r.Simulated.MakespanSeconds, r.Deadline) && within(r.Simulated.Cost, r.Budget)) || r.DeadlineMet != within(r.Simulated.MakespanSeconds, r.Deadline) || r.BudgetMet != within(r.Simulated.Cost, r.Budget) {
		panic("SLA classification mismatch")
	}
	if r.Algorithm == "heft" && (r.ScheduleSHA != base.ScheduleSHA || !within(math.Abs(r.Simulated.MakespanSeconds-base.Simulated.MakespanSeconds), 1e-6) || !within(math.Abs(r.Simulated.Cost-base.Simulated.Cost), 1e-9)) {
		panic("HEFT baseline changed across limits")
	}
	selectionPath := filepath.Join(filepath.Dir(r.PlanPath), "candidate-selection.json")
	var raw map[string]any
	if !load(selectionPath, &raw) {
		panic("missing pre-replay selection evidence")
	}
	expectedConfiguration := c.schedulerConfiguration(r.Algorithm, r.SLAAware)
	corrections := 0
	if !equalJSON(raw["configuration"], expectedConfiguration) {
		panic("recorded scheduler configuration differs from the frozen protocol")
	}
	encoded, e := json.Marshal(raw)
	must(e)
	var evidence struct {
		Objective  string           `json:"objective"`
		Selected   string           `json:"selected_original_id"`
		Eligible   int              `json:"eligible_count"`
		Count      int              `json:"candidate_count"`
		Filter     bool             `json:"filter_predicted_sla"`
		Before     bool             `json:"selected_before_native_replay"`
		Candidates []candidateScore `json:"candidates"`
	}
	must(json.Unmarshal(encoded, &evidence))
	filter := r.Algorithm != "heft" && (r.SLAAware || strings.HasPrefix(r.Algorithm, "moheft-"))
	if !evidence.Before || evidence.Objective != r.Panel || evidence.Filter != filter || evidence.Count != len(evidence.Candidates) || evidence.Count != r.CandidateCount {
		panic("selection policy differs from protocol")
	}
	plans := []domain.SchedulePlan{}
	selectedScore := domain.PredictedMetrics{}
	for _, score := range evidence.Candidates {
		if len(score.SHA) != 64 {
			panic("candidate hash missing")
		}
		plans = append(plans, domain.SchedulePlan{ID: score.ID, Predicted: score.Predicted})
		if score.ID == evidence.Selected {
			selectedScore = score.Predicted
		}
	}
	choice, eligible, bestEffort := selectPlan(plans, r.Panel, r.Deadline, r.Budget, filter)
	if choice.ID != evidence.Selected || eligible != evidence.Eligible || eligible != r.EligibleCandidates || bestEffort != r.BestEffort || selectedScore.MakespanSeconds != r.Predicted.MakespanSeconds || selectedScore.Cost != r.Predicted.Cost {
		panic("selected candidate was not the pre-replay predicted policy choice")
	}
	return corrections
}

func (c *campaign) verify() {
	expected := map[string]bool{}
	for _, s := range scenarios {
		for _, p := range []string{"time", "cost"} {
			for _, m := range margins {
				for _, a := range []string{"heft", "moheft-" + p, "prism-" + p} {
					expected[caseKey(s.ID, p, m, a, strings.HasPrefix(a, "prism-"))] = true
				}
				expected[caseKey(s.ID, p, m, "prism-"+p, false)] = true
			}
		}
	}
	baselines := map[string]result{}
	for _, s := range scenarios {
		var r result
		if !load(filepath.Join(c.Root, "baselines", s.ID, "result.json"), &r) {
			panic("baseline missing")
		}
		c.auditStored(r, result{})
		baselines[s.ID] = r
	}
	fallback := baselines["S4"].Simulated.Cost
	if fallback <= 0 {
		panic("invalid monetary reference")
	}
	paths, e := filepath.Glob(filepath.Join(c.Root, "cases", "*", "result.json"))
	must(e)
	rows := []result{}
	byID := map[string]result{}
	seenRuns := map[string]bool{}
	main, off, corrections := 0, 0, 0
	for _, path := range paths {
		var r result
		load(path, &r)
		if !expected[r.ID] || byID[r.ID].ID != "" || seenRuns[r.RunID] {
			panic("unexpected,duplicate or reused native execution")
		}
		if r.ID != caseKey(r.Environment, r.Panel, r.Margin, r.Algorithm, r.SLAAware) {
			panic("case identity mismatch")
		}
		base, ok := baselines[r.Environment]
		if !ok {
			panic("invalid environment")
		}
		b0 := base.Simulated.Cost
		reference := r.Environment
		if b0 <= 0 {
			b0 = fallback
			reference = "S4-positive-fallback"
		}
		d, b := base.Simulated.MakespanSeconds, b0
		if r.Panel == "time" {
			d *= r.Margin
		} else {
			b *= r.Margin
		}
		if r.Deadline != d || r.Budget != b || r.BudgetReference != reference || r.BudgetInformative != monetaryScope(c.inputs[r.Environment].Resources) || d <= 0 || b <= 0 {
			panic("limits or budget reference differ from frozen HEFT basis")
		}
		corrections += c.auditStored(r, base)
		if r.Algorithm == "heft" {
			shouldMeet := r.Margin >= 1
			if r.Panel == "cost" && base.Simulated.Cost == 0 {
				shouldMeet = true
			}
			if r.SLAMet != shouldMeet {
				panic("HEFT control does not match its own frozen reference")
			}
		}
		byID[r.ID] = r
		seenRuns[r.RunID] = true
		rows = append(rows, r)
		if strings.HasPrefix(r.Algorithm, "prism-") && !r.SLAAware {
			off++
		} else {
			main++
		}
	}
	if len(byID) != len(expected) || main != 210 || off != 70 {
		panic(fmt.Sprintf("incomplete exact grid main=%d/210 off=%d/70 unique=%d/280", main, off, len(byID)))
	}
	offSchedules := map[string]result{}
	for _, r := range rows {
		if !strings.HasPrefix(r.Algorithm, "prism-") || r.SLAAware {
			continue
		}
		key := r.Environment + "/" + r.Panel
		if previous, ok := offSchedules[key]; ok {
			if previous.ScheduleSHA != r.ScheduleSHA || !within(math.Abs(previous.Simulated.MakespanSeconds-r.Simulated.MakespanSeconds), 1e-6) || !within(math.Abs(previous.Simulated.Cost-r.Simulated.Cost), 1e-9) {
				panic("SLA-off changed scheduling/execution across margins")
			}
		} else {
			offSchedules[key] = r
		}
		on := byID[caseKey(r.Environment, r.Panel, r.Margin, r.Algorithm, true)]
		if on.InputSHA != r.InputSHA || on.Deadline != r.Deadline || on.Budget != r.Budget || !equalJSON(c.schedulerConfiguration(on.Algorithm, false), c.schedulerConfiguration(r.Algorithm, false)) {
			panic("uncontrolled ablation pair")
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	save(filepath.Join(c.Root, "results.json"), rows)
	summary, paired := summarize(rows)
	save(filepath.Join(c.Root, "summary.json"), summary)
	save(filepath.Join(c.Root, "ablation-summary.json"), paired)
	writeCSV(filepath.Join(c.Root, "observations.csv"), rows)
	mainRows, ablationRows := []result{}, []result{}
	for _, r := range rows {
		if !strings.HasPrefix(r.Algorithm, "prism-") || r.SLAAware {
			mainRows = append(mainRows, r)
		}
		if strings.HasPrefix(r.Algorithm, "prism-") {
			ablationRows = append(ablationRows, r)
		}
	}
	writeCSV(filepath.Join(c.Root, "comparison.csv"), mainRows)
	writeCSV(filepath.Join(c.Root, "ablation.csv"), ablationRows)
	save(filepath.Join(c.Root, "coverage-audit.json"), map[string]any{"audit_version": 2, "complete": true, "main_cases": main, "additional_sla_off_cases": off, "native_simulations": len(rows), "native_baselines": 7, "activities_each": 6448, "all_native_tasks_checked": (len(rows) + 7) * 6448, "exact_grid": true, "all_assignments_and_precedences_checked": true, "all_input_model_plan_native_hashes_checked": true, "pre_simulation_candidate_policy_checked": true, "unchanged_sla_off_schedules_checked": true, "paired_ablation_inputs_and_limits_checked": true, "metadata_configuration_corrections": corrections, "simulation_only": true})
	logf("verified exact grid main=%d off=%d; all%d native task traces audited", main, off, (len(rows)+7)*6448)
}
func summarize(rows []result) ([]aggregate, []ablation) {
	groups := map[string]*aggregate{}
	byID := map[string]result{}
	for _, r := range rows {
		byID[r.ID] = r
		if strings.HasPrefix(r.Algorithm, "prism-") && !r.SLAAware {
			continue
		}
		key := fmt.Sprintf("%s/%s/%.1f", r.Panel, r.Algorithm, r.Margin)
		g := groups[key]
		if g == nil {
			g = &aggregate{Panel: r.Panel, Algorithm: r.Algorithm, Margin: r.Margin}
			groups[key] = g
		}
		g.N++
		if r.SLAMet {
			g.Met++
		}
		if r.DeadlineMet {
			g.DeadlineMet++
		}
		if r.BudgetMet {
			g.BudgetMet++
		}
		if r.PredictionSaysFeasible {
			g.PredictedMet++
		}
		if r.PredictionSaysFeasible && !r.SLAMet {
			g.FalseFeasible++
		}
		if !r.PredictionSaysFeasible && r.SLAMet {
			g.FalseInfeasible++
		}
		if r.BudgetInformative {
			g.MonetaryN++
			if r.SLAMet {
				g.MonetaryMet++
			}
		}
	}
	out := []aggregate{}
	for _, g := range groups {
		if g.N != 7 {
			panic("wrong figure denominator")
		}
		g.Percent = 100 * float64(g.Met) / float64(g.N)
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Panel != b.Panel {
			return a.Panel < b.Panel
		}
		if a.Margin != b.Margin {
			return a.Margin < b.Margin
		}
		return a.Algorithm < b.Algorithm
	})
	paired := []ablation{}
	for _, panel := range []string{"time", "cost"} {
		for _, margin := range margins {
			a := ablation{Panel: panel, Margin: margin}
			for _, s := range scenarios {
				on := byID[caseKey(s.ID, panel, margin, "prism-"+panel, true)]
				off := byID[caseKey(s.ID, panel, margin, "prism-"+panel, false)]
				if on.ID == "" || off.ID == "" {
					panic("missing paired ablation")
				}
				a.N++
				if on.SLAMet {
					a.OnMet++
				}
				if off.SLAMet {
					a.OffMet++
				}
				if on.SLAMet && off.SLAMet {
					a.Both++
				} else if on.SLAMet {
					a.OnOnly++
				} else if off.SLAMet {
					a.OffOnly++
				} else {
					a.Neither++
				}
				if on.ScheduleSHA != off.ScheduleSHA {
					a.DifferentSchedules++
				}
			}
			paired = append(paired, a)
		}
	}
	return out, paired
}
func writeCSV(path string, rows []result) {
	f, e := os.Create(path)
	must(e)
	defer f.Close()
	w := csv.NewWriter(f)
	columns := []string{"id", "environment", "panel", "algorithm", "sla_aware", "margin", "deadline_seconds", "budget_usd", "budget_reference", "budget_informative", "predicted_seconds", "predicted_cost", "simulated_seconds", "simulated_cost", "predicted_sla_met", "deadline_met", "budget_met", "sla_met", "best_effort", "candidate_count", "eligible_candidates", "schedule_sha256", "plan_id", "run_id", "plan_path", "plan_sha256", "trace_path", "trace_sha256", "native_path", "native_sha256", "input_sha256", "model_sha256", "simgrid_binary_sha256", "planning_wall_seconds", "simulation_wall_seconds", "completed_activities", "verified_relations", "candidate_origin"}
	columns = append(columns, "algorithm_version", "predicted_cost_basis", "simulated_cost_basis")
	must(w.Write(columns))
	str := func(v any) string {
		if v == nil {
			return ""
		}
		if n, ok := v.(float64); ok {
			return strconv.FormatFloat(n, 'g', 17, 64)
		}
		return fmt.Sprint(v)
	}
	for _, r := range rows {
		values := []any{r.ID, r.Environment, r.Panel, r.Algorithm, r.SLAAware, r.Margin, r.Deadline, r.Budget, r.BudgetReference, r.BudgetInformative, r.Predicted.MakespanSeconds, r.Predicted.Cost, r.Simulated.MakespanSeconds, r.Simulated.Cost, r.PredictionSaysFeasible, r.DeadlineMet, r.BudgetMet, r.SLAMet, r.BestEffort, r.CandidateCount, r.EligibleCandidates, r.ScheduleSHA, r.PlanID, r.RunID, r.PlanPath, r.PlanSHA, r.TracePath, r.TraceSHA, r.NativePath, r.NativeSHA, r.InputSHA, r.ModelSHA, r.BinarySHA, r.PlanningSeconds, r.SimulationSeconds, r.Completed, r.VerifiedRelations, r.CandidateOrigin}
		var nativeInfo struct {
			Version  string         `json:"algorithmVersion"`
			Metadata map[string]any `json:"metadata"`
		}
		load(r.PlanPath, &nativeInfo)
		basis := nativeInfo.Metadata["costModel"]
		if basis == nil {
			basis = "native scheduler forecast with lifecycle enrichment"
		}
		values = append(values, nativeInfo.Version, basis, "common native SimGrid VM lifecycle compute/disk plus modeled network tariffs")
		line := []string{}
		for _, v := range values {
			line = append(line, str(v))
		}
		must(w.Write(line))
	}
	w.Flush()
	must(w.Error())
}
