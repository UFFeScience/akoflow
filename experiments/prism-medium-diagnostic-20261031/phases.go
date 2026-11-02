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

func read(p string) M {
	b, e := os.ReadFile(p)
	if e != nil {
		panic(e)
	}
	var m M
	if e = json.Unmarshal(b, &m); e != nil {
		panic(e)
	}
	return m
}
func n(x any) float64 { v, _ := x.(float64); return v }
func s(x any) string  { v, _ := x.(string); return v }
func m(x any) M {
	v, _ := x.(map[string]any)
	if v == nil {
		return M{}
	}
	return v
}
func rows(x any) []M {
	a, _ := x.([]any)
	r := []M{}
	for _, v := range a {
		r = append(r, m(v))
	}
	return r
}
func main() {
	root := "outputs/fig8-canonical-20261031/prism-time-followup/"
	out := "outputs/prism-medium-investigation-20261031/"
	req := read(root + "medium-request.json")
	real := read(root + "medium-final-monitor.json")
	sim := read(out + "simulation-monitor.json")
	plan := read(root + "medium-plan.json")
	pred := map[string][]string{}
	for _, d := range rows(m(req["Workflow"])["dependencies"]) {
		id := s(d["activityId"])
		pred[id] = append(pred[id], s(d["dependsOnActivityId"]))
	}
	lanes := map[string][]M{}
	lanePred := map[string][]string{}
	for _, a := range rows(plan["assignments"]) {
		key := s(a["coreId"])
		lanes[key] = append(lanes[key], a)
	}
	for _, lane := range lanes {
		sort.Slice(lane, func(i, j int) bool { return n(lane[i]["orderOnResource"]) < n(lane[j]["orderOnResource"]) })
		for i := 1; i < len(lane); i++ {
			id := s(lane[i]["activityId"])
			lanePred[id] = append(lanePred[id], s(lane[i-1]["activityId"]))
		}
	}
	maps := []map[string]M{{}, {}}
	origins := []float64{math.Inf(1), math.Inf(1)}
	monitors := []M{sim, real}
	for k, monitor := range monitors {
		for _, a := range rows(monitor["activities"]) {
			maps[k][s(a["activityId"])] = a
			origins[k] = math.Min(origins[k], n(a["startedAt"]))
		}
		if len(maps[k]) != 58 {
			panic("expected 58 unique activities")
		}
	}
	records := []M{}
	laneDivergences := []M{}
	for id, predecessors := range lanePred {
		for _, previous := range predecessors {
			for k, taskMap := range maps {
				if n(taskMap[previous]["finishedAt"]) > n(taskMap[id]["startedAt"])+1e-4 {
					laneDivergences = append(laneDivergences, M{"mode": []string{"simulation", "real"}[k], "predecessor": previous, "successor": id, "predecessorFinish": n(taskMap[previous]["finishedAt"]) - origins[k], "successorStart": n(taskMap[id]["startedAt"]) - origins[k]})
				}
			}
		}
	}
	totals := []map[string]M{{}, {}}
	for id, sa := range maps[0] {
		ra := maps[1][id]
		if ra == nil || sa["status"] != "completed" || ra["status"] != "completed" || sa["allocatedResourceId"] != ra["allocatedResourceId"] {
			panic("missing completed activity or mismatched placement")
		}
		sm, rm := m(sa["metadata"]), m(ra["metadata"])
		logical := []float64{0, origins[1]}
		for _, d := range rows(m(req["Workflow"])["dependencies"]) {
			if s(d["activityId"]) == id {
				pid := s(d["dependsOnActivityId"])
				for k := 0; k < 2; k++ {
					logical[k] = math.Max(logical[k], n(maps[k][pid]["finishedAt"]))
				}
			}
		}
		records = append(records, M{"activity": id, "resource": ra["allocatedResourceId"], "simStart": n(sa["startedAt"]) - origins[0], "realStart": n(ra["startedAt"]) - origins[1], "simFinish": n(sa["finishedAt"]) - origins[0], "realFinish": n(ra["finishedAt"]) - origins[1], "simCompute": sa["runtimeSeconds"], "realCompute": ra["runtimeSeconds"], "simLogicalReadyToStart": n(sa["startedAt"]) - logical[0], "realLogicalReadyToStart": n(ra["startedAt"]) - logical[1], "simPreparationStage": sm["workspacePreparationSeconds"], "simFullInputReadyToPreparationDone": n(sm["workspacePreparationFinishedAt"]) - logical[0], "realPreparationElapsed": n(rm["preparationFinishedAt"]) - n(rm["preparationStartedAt"]), "realPostPreparationWait": n(ra["startedAt"]) - n(rm["preparationFinishedAt"])})
		for k, a := range []M{sa, ra} {
			res := s(a["allocatedResourceId"])
			t := totals[k][res]
			if t == nil {
				t = M{"count": float64(0), "compute": float64(0)}
				totals[k][res] = t
			}
			t["count"] = n(t["count"]) + 1
			t["compute"] = n(t["compute"]) + n(a["runtimeSeconds"])
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return n(records[i]["realFinish"])-n(records[i]["simFinish"]) > n(records[j]["realFinish"])-n(records[j]["simFinish"])
	})
	chains := []M{}
	for k, taskMap := range maps {
		last := ""
		lastTime := -1.0
		violations := 0
		for id, a := range taskMap {
			if n(a["finishedAt"]) > lastTime {
				last = id
				lastTime = n(a["finishedAt"])
			}
			for _, p := range pred[id] {
				if n(taskMap[p]["finishedAt"]) > n(a["startedAt"])+1e-4 {
					violations++
				}
			}
		}
		path := []M{}
		compute, wait := 0.0, 0.0
		seen := map[string]bool{}
		for id := last; id != ""; {
			if seen[id] {
				panic("cycle")
			}
			seen[id] = true
			a := taskMap[id]
			gate := origins[k]
			previous := ""
			for _, p := range pred[id] {
				if n(taskMap[p]["finishedAt"]) > gate {
					gate = n(taskMap[p]["finishedAt"])
					previous = p
				}
			}
			local := n(a["finishedAt"]) - gate
			runtime := n(a["runtimeSeconds"])
			compute += runtime
			wait += local - runtime
			path = append(path, M{"activity": id, "gateActivity": previous, "localElapsed": local, "compute": runtime, "otherElapsed": local - runtime})
			id = previous
		}
		if violations != 0 || math.Abs(compute+wait-(lastTime-origins[k])) > 1e-5 {
			panic("invalid workflow precedence ledger")
		}
		chains = append(chains, M{"mode": []string{"simulation", "real"}[k], "note": "workflow-dependency timestamp ledger only; residual includes resource waiting and all other elapsed service, not isolated causal attribution", "edgeTimingViolations": violations, "makespan": lastTime - origins[k], "pathCompute": compute, "pathOtherElapsed": wait, "sum": compute + wait, "reversePath": path})
	}
	result := M{"activityComparison": records, "resourceComputeTotals": totals, "precedenceLedgers": chains, "planLaneOrderDivergences": laneDivergences}
	b, e := json.MarshalIndent(result, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(out+"phase-ledger.json", b, 0644); e != nil {
		panic(e)
	}
	for _, c := range chains {
		fmt.Printf("%s makespan %.2f compute %.2f other %.2f violations %v\n", c["mode"], c["makespan"], c["pathCompute"], c["pathOtherElapsed"], c["edgeTimingViolations"])
	}
	for _, a := range records {
		if strings.Contains(s(a["activity"]), "mconcatfit") || strings.Contains(s(a["activity"]), "mviewerid0000058") {
			b, _ := json.Marshal(a)
			fmt.Println(string(b))
		}
	}
}
