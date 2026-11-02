//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

type M = map[string]any

const dir = "outputs/prism-time-medium-strict-20261031/"

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func read(p string) M {
	b, e := os.ReadFile(p)
	must(e)
	var v M
	must(json.Unmarshal(b, &v))
	return v
}
func m(x any) M {
	v, _ := x.(map[string]any)
	if v == nil {
		return M{}
	}
	return v
}
func n(x any) float64 { v, _ := x.(float64); return v }
func s(x any) string  { v, _ := x.(string); return v }
func rows(x any) []M {
	a, _ := x.([]any)
	out := []M{}
	for _, v := range a {
		out = append(out, m(v))
	}
	return out
}
func ledger(trace, plan M) M {
	if m(trace["run"])["status"] != "completed" {
		panic("completed trace required")
	}
	tasks := map[string]M{}
	first, last := math.Inf(1), 0.0
	for _, a := range rows(trace["activities"]) {
		tasks[s(a["activityId"])] = a
		first = math.Min(first, n(a["startedAt"]))
		last = math.Max(last, n(a["finishedAt"]))
	}
	selected := []M{}
	core := ""
	for _, a := range rows(plan["assignments"]) {
		if !strings.Contains(s(a["resourceId"]), "diablo03") {
			continue
		}
		if core != "" && core != s(a["coreId"]) {
			panic("HPC is not one serial lane")
		}
		core = s(a["coreId"])
		selected = append(selected, a)
	}
	if len(selected) != 30 {
		panic("expected 30 HPC assignments")
	}
	sort.Slice(selected, func(i, j int) bool { return n(selected[i]["orderOnResource"]) < n(selected[j]["orderOnResource"]) })
	compute, wallOther, gaps := 0.0, 0.0, 0.0
	previous := 0.0
	for i, a := range selected {
		task := tasks[s(a["activityId"])]
		if task == nil || task["status"] != "completed" || task["allocatedResourceId"] != a["resourceId"] {
			panic("missing completed matching HPC task")
		}
		start, finish, runtime := n(task["startedAt"]), n(task["finishedAt"]), n(task["runtimeSeconds"])
		if i == 0 && math.Abs(start-first) > 1e-5 {
			panic("HPC lane does not start workflow makespan")
		}
		if i > 0 {
			gap := start - previous
			if gap < -1e-5 {
				panic("HPC lane overlaps or reorders")
			}
			gaps += gap
		}
		if finish-start-runtime < -1e-5 {
			panic("runtime exceeds task wall interval")
		}
		compute += runtime
		wallOther += finish - start - runtime
		previous = finish
	}
	if math.Abs(previous-last) > 1e-5 {
		panic("HPC lane does not end workflow makespan")
	}
	total := compute + wallOther + gaps
	reported := n(m(trace["run"])["makespanSeconds"])
	if math.Abs(total-reported) > 1e-5 {
		panic("serial accounting does not reconcile with makespan")
	}
	return M{"hpcActivityCount": len(selected), "logicalCore": core, "computeSeconds": compute, "otherTaskWallSeconds": wallOther, "interTaskGapSeconds": gaps, "allNonComputeSeconds": wallOther + gaps, "makespanSeconds": reported, "reconstructedSeconds": total, "identityResidualSeconds": total - reported, "passed": true}
}
func main() {
	if read(dir + "strict-order-audit.json")["Complete"] != true {
		panic("strict audit incomplete")
	}
	p := read(dir + "medium-plan.json")
	sim := ledger(read(dir+"simulation-monitor.json"), p)
	real := ledger(read(dir+"strict-live-monitor.json"), p)
	difference := n(real["makespanSeconds"]) - n(sim["makespanSeconds"])
	computeDifference := n(real["computeSeconds"]) - n(sim["computeSeconds"])
	otherDifference := n(real["allNonComputeSeconds"]) - n(sim["allNonComputeSeconds"])
	if math.Abs(difference-computeDifference-otherDifference) > 1e-5 {
		panic("difference reconciliation failed")
	}
	report := M{"simulated": sim, "strictReal": real, "realMinusSimulatedSeconds": difference, "computeDifferenceSeconds": computeDifference, "nonComputeDifferenceSeconds": otherDifference, "note": "Exact elapsed-time accounting on the single HPC lane covering first and last workflow activities. Non-compute includes transfer/preparation, data readiness, Slurm, launch, observation and polling; this does not isolate causal link service or sum parallel transfer work."}
	b, e := json.MarshalIndent(report, "", "  ")
	must(e)
	must(os.WriteFile(dir+"serial-hpc-accounting.json", b, 0600))
	fmt.Println(string(b))
}
