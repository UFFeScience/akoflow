package montagetools

import (
	"sort"
	"strings"
)

func dependencyReference(real, sim Object) Object {
	check(obj(get(real, "run"))["status"] == "completed", "completed real reference required")
	latest := map[string]Object{}
	for _, transfer := range rows(get(real, "artifactTransferRuns")) {
		if transfer["status"] != "completed" {
			continue
		}
		route := obj(transfer["route"])
		if route["producerActivityId"] == nil || route["consumerActivityId"] == nil {
			continue
		}
		key := str(route["producerActivityId"]) + "\x00" + str(route["consumerActivityId"])
		if latest[key] == nil || num(transfer["startedAt"]) > num(latest[key]["startedAt"]) {
			latest[key] = transfer
		}
	}
	predicted := map[string]Object{}
	for _, transfer := range rows(get(sim, "transfers")) {
		predicted[str(get(transfer, "producerActivityId"))+"\x00"+str(get(transfer, "consumerActivityId"))] = transfer
	}
	records := []any{}
	for key, transfer := range latest {
		route := obj(transfer["route"])
		row := Object{"producer": route["producerActivityId"], "consumer": route["consumerActivityId"], "observedStrategy": transfer["strategy"], "observedNetworkBytes": transfer["networkBytes"]}
		if transfer["strategy"] == "use-existing" {
			row["comparable"] = false
			row["reason"] = "recovery reuses validated copies; no fresh transfer duration"
		} else {
			var wall float64
			for _, phase := range rows(route["phases"]) {
				if !strings.HasPrefix(str(get(phase, "name")), "shared-") {
					wall += num(phase["durationSeconds"])
				}
			}
			if transfer["strategy"] == "runtime-local" {
				wall = 0
			}
			row["observedDependencyWallSeconds"] = wall
			estimate, exists := predicted[key]
			row["comparable"] = exists
			if exists {
				row["predictedStrategy"] = estimate["strategy"]
				row["predictedNetworkBytes"] = estimate["networkBytes"]
				row["predictedDependencySeconds"] = estimate["durationSeconds"]
				row["durationErrorPercent"] = errorPercent(num(estimate["durationSeconds"]), wall)
			}
		}
		records = append(records, row)
	}
	sort.Slice(records, func(i, j int) bool {
		return num(obj(records[i])["observedDependencyWallSeconds"]) > num(obj(records[j])["observedDependencyWallSeconds"])
	})
	return Object{"referenceRunId": obj(real["run"])["id"], "role": "recovered-reference-residuals-not-independent-validation", "dependencies": records, "limitations": list("Shared consumer barrier and merge phases are excluded from each dependency to avoid duplicate counting.", "Observed phase wall time can contain admission wait; this is not queue-free service calibration.", "Recovery copy reuse is explicitly excluded from fresh duration comparisons.", "No parallel dependency durations are summed as makespan.")}
}
