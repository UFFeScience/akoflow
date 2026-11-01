//go:build ignore

package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
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
func obj(v any) M {
	m, _ := v.(map[string]any)
	if m == nil {
		return M{}
	}
	return m
}
func num(v any) float64 { n, _ := v.(float64); return n }
func str(v any) string  { s, _ := v.(string); return s }
func rows(v any) []M {
	a, _ := v.([]any)
	r := []M{}
	for _, v := range a {
		r = append(r, obj(v))
	}
	return r
}
func value(v any) string {
	if v == nil {
		return ""
	}
	n, ok := v.(float64)
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
		panic("invalid measurement")
	}
	return strconv.FormatFloat(n, 'f', 9, 64)
}
func writeCSV(name string, header []string, data [][]string) {
	f, e := os.OpenFile(dir+name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	must(e)
	w := csv.NewWriter(f)
	must(w.Write(header))
	must(w.WriteAll(data))
	must(w.Error())
	must(f.Close())
}
func main() {
	audit := read(dir + "strict-order-audit.json")
	if audit["Complete"] != true {
		panic("strict audit not complete")
	}
	old := read("outputs/fig8-canonical-20261031/prism-time-followup/medium-final-monitor.json")
	fresh := read(dir + "strict-live-monitor.json")
	sim := read(dir + "simulation-monitor.json")
	plan := read(dir + "medium-plan.json")
	p := num(obj(plan["predicted"])["makespanSeconds"])
	s := num(obj(sim["run"])["makespanSeconds"])
	summary := []M{}
	table := [][]string{}
	for i, monitor := range []M{old, fresh} {
		run := obj(monitor["run"])
		if run["status"] != "completed" || len(rows(monitor["activities"])) != 58 {
			panic("incomplete observation")
		}
		r := num(run["makespanSeconds"])
		cost := obj(monitor["cloudBilling"])
		for _, a := range rows(monitor["activities"]) {
			if a["status"] != "completed" || num(a["attempt"]) != 1 {
				panic("unexpected retry or failed activity")
			}
		}
		label := []string{"previous-flexible-order", "strict-planned-order"}[i]
		entry := M{"cohort": label, "runId": run["id"], "plannedSeconds": p, "simulatedSeconds": s, "realSeconds": r, "simulationAbsoluteErrorPercent": 100 * math.Abs(s-r) / r, "realTransferredBytes": run["transferredBytes"], "realTransferWorkSeconds": obj(run["breakdown"])["transferSeconds"], "cloudLifecycleEstimatedUSD": cost["estimatedCost"], "costBasis": cost["costBasis"], "recovery": false}
		summary = append(summary, entry)
		table = append(table, []string{label, str(run["id"]), value(p), value(s), value(r), value(entry["simulationAbsoluteErrorPercent"]), value(entry["realTransferredBytes"]), value(entry["realTransferWorkSeconds"]), value(entry["cloudLifecycleEstimatedUSD"])})
	}
	writeCSV("comparison.csv", []string{"cohort", "run_id", "planned_seconds", "simulated_seconds", "real_seconds", "simulation_absolute_error_percent_real_denominator", "real_transferred_bytes", "real_transfer_work_seconds_not_elapsed", "cloud_lifecycle_estimated_usd"}, table)
	indexed := []map[string]M{{}, {}, {}}
	origins := []float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	for k, monitor := range []M{sim, old, fresh} {
		for _, a := range rows(monitor["activities"]) {
			indexed[k][str(a["activityId"])] = a
			origins[k] = math.Min(origins[k], num(a["startedAt"]))
		}
	}
	handles := map[string]M{}
	for _, h := range rows(fresh["handles"]) {
		handles[str(h["activityId"])] = h
	}
	assignments := rows(plan["assignments"])
	sort.Slice(assignments, func(i, j int) bool { return str(assignments[i]["activityId"]) < str(assignments[j]["activityId"]) })
	activities := [][]string{}
	for _, a := range assignments {
		id := str(a["activityId"])
		record := []string{id, str(a["resourceId"]), str(a["coreId"]), value(a["orderOnResource"]), value(a["predictedStartAt"]), value(a["predictedFinishAt"])}
		for k := range indexed {
			task := indexed[k][id]
			if task == nil {
				panic("missing matched activity")
			}
			meta := obj(task["metadata"])
			record = append(record, value(num(task["startedAt"])-origins[k]), value(num(task["finishedAt"])-origins[k]), value(task["runtimeSeconds"]))
			prep := any(nil)
			if k == 0 {
				prep = meta["workspacePreparationSeconds"]
			} else if meta["preparationFinishedAt"] != nil && meta["preparationStartedAt"] != nil {
				prep = num(meta["preparationFinishedAt"]) - num(meta["preparationStartedAt"])
			}
			record = append(record, value(prep))
		}
		h := handles[id]
		meta := obj(h["metadata"])
		queue := any(nil)
		if meta["submittedAt"] != nil {
			queue = num(h["startedAt"]) - num(meta["submittedAt"])
		}
		record = append(record, value(queue))
		activities = append(activities, record)
	}
	writeCSV("activities.csv", []string{"activity_id", "resource_id", "logical_core_id", "order_on_resource", "planned_start_s", "planned_finish_s", "sim_start_s", "sim_finish_s", "sim_compute_s", "sim_preparation_stage_s", "legacy_start_s", "legacy_finish_s", "legacy_compute_s", "legacy_preparation_elapsed_s", "strict_start_s", "strict_finish_s", "strict_compute_s", "strict_preparation_elapsed_s", "strict_slurm_admission_s"}, activities)
	report := M{"comparison": summary, "strictOrderAudit": audit, "sameInputAudit": read(dir + "input-equivalence-audit.json"), "notes": []string{"Service rates and external conditions can vary between runs; differences are descriptive, not isolated causal effects of strict ordering.", "Simulated preparation stage excludes preceding transfer stages; it is not directly comparable to the entire real preparation interval.", "Aggregate transfer work overlaps; it is not elapsed transfer time or additive critical-path decomposition.", "Cloud lifecycle is an estimate of VM plus boot disk, not a provider invoice; HPC and excluded costs remain unpriced."}}
	b, e := json.MarshalIndent(report, "", "  ")
	must(e)
	must(os.WriteFile(dir+"comparison.json", b, 0600))
	for _, r := range table {
		fmt.Println(r)
	}
}
