package montagetools

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func audit(request, response, simulation Object, fresh bool) Object {
	run := obj(get(response, "run"))
	check(run["status"] == "completed", "Run is not completed")
	expected := Object{}
	for _, a := range rows(get(obj(get(request, "workflow")), "activities")) {
		id := str(get(a, "id"))
		check(expected[id] == nil, "Duplicate expected activity IDs")
		expected[id] = true
	}
	assignments := index(get(obj(get(request, "plan")), "assignments"), "activityId")
	latest := Object{}
	counts := map[string]int{}
	multiple := false
	tasks := rows(get(response, "activityAttempts"))
	handles := rows(response["handles"])
	for _, t := range tasks {
		id := str(get(t, "activityId"))
		counts[id]++
		old := obj(latest[id])
		if old["attempt"] == nil || num(t["attempt"]) > num(old["attempt"]) || (num(t["attempt"]) == num(old["attempt"]) && num(t["startedAt"]) > num(old["startedAt"])) {
			latest[id] = t
		}
		multiple = multiple || counts[id] > 1 || num(t["attempt"]) > 1
	}
	exactHandle := func(id string, task Object) Object {
		canonical := str(run["id"]) + ":" + id
		if num(task["attempt"]) > 1 {
			canonical += fmt.Sprintf(":attempt-%.0f", num(task["attempt"]))
		}
		var best Object
		for _, h := range handles {
			attempt := obj(h["metadata"])["executionAttempt"]
			if h["activityId"] == id && (eq(attempt, task["attempt"]) || (attempt == nil && h["id"] == canonical && h["runId"] == run["id"])) {
				if best == nil || num(h["startedAt"]) > num(best["startedAt"]) {
					best = h
				}
			}
		}
		return best
	}
	if fresh {
		check(len(tasks) == 58 && len(handles) == 58, "Unexpected attempts or runtime handles in fresh experiment")
		for _, t := range tasks {
			check(num(t["attempt"]) == 1 && t["status"] == "completed", "Fresh experiment contains retries or failed attempts")
		}
		for id := range expected {
			hs := []Object{}
			for _, h := range handles {
				a := obj(h["metadata"])["executionAttempt"]
				if h["id"] == str(run["id"])+":"+id && h["runId"] == run["id"] && h["activityId"] == id && (a == nil || num(a) == 1) {
					hs = append(hs, h)
				}
			}
			check(len(hs) == 1 && hs[0]["status"] == "completed" && hs[0]["exitCode"] != nil && num(hs[0]["exitCode"]) == 0, "Missing successful exact-attempt handle: "+id)
		}
		b := obj(get(response, "cloudBilling"))
		check(b["status"] == "estimated" && b["currency"] == "USD" && num(b["estimatedCost"]) > 0, "Cloud lifecycle prices are incomplete")
	}
	transfers := rows(get(response, "artifactTransferRuns"))
	deps := []Object{}
	for _, t := range transfers {
		if obj(t["route"])["producerActivityId"] != nil {
			deps = append(deps, t)
		}
	}
	types := Object{}
	for _, r := range rows(request["resources"]) {
		types[str(r["id"])] = r["type"]
	}
	missingRoutes, cross, nonRelay, incomplete, barriers, leases, overlaps := 0, 0, 0, 0, 0, 0, 0
	for _, e := range rows(obj(request["workflow"])["dependencies"]) {
		found := false
		for _, t := range deps {
			r := obj(t["route"])
			found = found || (r["producerActivityId"] == e["dependsOnActivityId"] && r["consumerActivityId"] == e["activityId"])
		}
		if !found {
			missingRoutes++
		}
	}
	for _, t := range deps {
		r := obj(t["route"])
		s := types[str(obj(get(assignments, str(r["producerActivityId"])))["resourceId"])]
		target := types[str(obj(get(assignments, str(r["consumerActivityId"])))["resourceId"])]
		isHPC := func(v any) bool { return v == "hpc_machine" || v == "hpc_partition" }
		if (s == "cloud_vm" && isHPC(target)) || (target == "cloud_vm" && isHPC(s)) {
			cross++
			if t["strategy"] != "gateway" {
				nonRelay++
			}
		}
	}
	for _, t := range transfers {
		if t["status"] != "completed" {
			incomplete++
		}
		task := obj(latest[str(t["activityId"])])
		if task["startedAt"] != nil && num(t["finishedAt"]) > num(task["startedAt"]) {
			barriers++
		}
	}
	for _, l := range rows(get(response, "workspaceLeases")) {
		if l["released"] != true {
			leases++
		}
	}
	for _, r := range rows(request["resources"]) {
		id := str(r["id"])
		if !strings.Contains(id, "micro") || num(r["cpuCapacity"]) != 1 {
			continue
		}
		intervals := []Object{}
		for key, v := range latest {
			if obj(assignments[key])["resourceId"] == id {
				t := obj(v)
				check(t["startedAt"] != nil && t["finishedAt"] != nil && num(t["finishedAt"]) >= num(t["startedAt"]), "Missing or invalid micro execution timestamps")
				intervals = append(intervals, t)
			}
		}
		sort.SliceStable(intervals, func(i, j int) bool { return num(intervals[i]["startedAt"]) < num(intervals[j]["startedAt"]) })
		for i := 1; i < len(intervals); i++ {
			if num(intervals[i-1]["finishedAt"]) > num(intervals[i]["startedAt"]) {
				overlaps++
			}
		}
	}
	missing, unexpected, notCompleted := []any{}, []any{}, []any{}
	for _, id := range sortedKeys(expected) {
		if latest[id] == nil {
			missing = append(missing, id)
		}
		if obj(latest[id])["status"] != "completed" {
			notCompleted = append(notCompleted, id)
		}
	}
	for _, id := range sortedKeys(latest) {
		if expected[id] == nil {
			unexpected = append(unexpected, id)
		}
	}
	reportAudit := Object{"expected_count": len(expected), "missing": missing, "unexpected": unexpected, "not_completed": notCompleted, "incomplete_transfer_count": incomplete, "barrier_violation_count": barriers, "unreleased_lease_count": leases, "single_lane_micro_overlap_count": overlaps, "missing_dependency_route_count": missingRoutes, "cross_cloud_hpc_non_relay_count": nonRelay}
	check(len(expected) == 58 && len(missing) == 0 && len(unexpected) == 0 && len(notCompleted) == 0 && incomplete+barriers+leases+overlaps+missingRoutes+nonRelay == 0, "Activity/transfer audit failed")
	metrics := func(m Object) Object { return pick(m, "makespanSeconds", "cost", "transferredBytes", "breakdown") }
	network, relay := 0.0, 0.0
	routeRows := []any{}
	for _, t := range transfers {
		r := obj(t["route"])
		network += num(t["networkBytes"])
		relay += num(r["serverNetworkBytes"])
		o := Object{"consumer_activity_id": t["activityId"], "producer_activity_id": r["producerActivityId"], "strategy": t["strategy"], "duration_seconds": t["durationSeconds"], "logical_bytes": t["logicalBytes"], "network_bytes": t["networkBytes"], "relay_server_network_bytes": r["serverNetworkBytes"], "archive_bytes": r["archiveBytes"], "files": r["filesTransferred"], "snapshot_reused": r["snapshotReused"], "shared_storage_proof_failure": r["sharedStorageProofFailure"], "ssh_channel_wait_seconds": r["sshChannelWaitSeconds"], "rsync_execution_seconds": r["rsyncExecutionSeconds"], "source_pack_preflight_seconds": r["sourcePackPreflightSeconds"], "source_pack_preflight_ssh_channel_wait_seconds": r["sourcePackPreflightSshChannelWaitSeconds"], "source_pack_preflight_service_seconds": r["sourcePackPreflightServiceSeconds"], "source_pack_ssh_channel_wait_seconds": r["sourcePackSshChannelWaitSeconds"], "source_pack_execution_seconds": r["sourcePackExecutionSeconds"]}
		phases := []any{}
		for _, p := range rows(r["phases"]) {
			phases = append(phases, pick(p, "name", "durationSeconds", "sshChannelWaitSeconds", "executionSeconds"))
		}
		o["phases"] = phases
		routeRows = append(routeRows, o)
	}
	shared := []any{}
	seen := Object{}
	for _, t := range deps {
		r := obj(t["route"])
		var barrier, merge Object
		for _, p := range rows(r["phases"]) {
			if p["name"] == "shared-preparation-barrier" {
				barrier = p
			}
			if p["name"] == "shared-conflict-check-copy-and-verify" {
				merge = p
			}
		}
		if barrier["startedAt"] == nil || barrier["finishedAt"] == nil {
			continue
		}
		key := fmt.Sprint(r["consumerActivityId"], ":", barrier["startedAt"], ":", barrier["finishedAt"])
		if seen[key] != nil {
			continue
		}
		seen[key] = true
		shared = append(shared, Object{"consumer_activity_id": r["consumerActivityId"], "barrier_seconds": barrier["durationSeconds"], "merge_seconds": merge["durationSeconds"], "merge_check_seconds": r["mergeCheckSeconds"], "merge_copy_seconds": r["mergeCopySeconds"], "merge_verify_seconds": r["mergeVerifySeconds"], "merge_ssh_channel_wait_seconds": r["mergeSshChannelWaitSeconds"], "merge_execution_seconds": r["mergeExecutionSeconds"], "merge_command_overhead_seconds": r["mergeCommandOverheadSeconds"]})
	}
	activityRows := []any{}
	for _, id := range sortedKeys(expected) {
		t, a := obj(latest[id]), obj(get(assignments, id))
		h := exactHandle(id, t)
		metadata := obj(t["metadata"])
		observed := obj(h["metadata"])["terminalObservedAt"]
		var runtime, delay any
		if observed != nil && h["finishedAt"] != nil && h["startedAt"] != nil && num(observed) >= num(h["finishedAt"]) && num(h["finishedAt"]) >= num(h["startedAt"]) {
			runtime = num(h["finishedAt"]) - num(h["startedAt"])
			delay = num(observed) - num(h["finishedAt"])
		}
		activityRows = append(activityRows, Object{"activity_id": id, "resource_id": a["resourceId"], "attempt": t["attempt"], "planned_compute_seconds": a["predictedRuntimeSeconds"], "planned_transfer_seconds": a["predictedTransferSeconds"], "observed_runtime_seconds": t["runtimeSeconds"], "validated_handle_runtime_seconds": runtime, "terminal_observation_delay_seconds": delay, "preparation_elapsed_seconds": metadata["transferElapsedSeconds"], "overlapping_transfer_work_seconds": metadata["transferWorkSeconds"], "launch_wait_seconds": metadata["launchWaitSeconds"], "queue_seconds": t["queueSeconds"], "network_bytes": t["transferBytes"]})
	}
	history, attemptLimitation := "single_observed_attempt_per_activity", "No multiple attempts observed; this alone does not prove an uninterrupted benchmark."
	if multiple {
		history, attemptLimitation = "multiple_attempts", "Multiple attempts: performance can be contaminated by recovery."
	}
	var wall any
	if run["startedAt"] != nil && run["finishedAt"] != nil {
		s, e := time.Parse(time.RFC3339Nano, str(run["startedAt"]))
		check(e == nil, "invalid run start timestamp")
		f, e := time.Parse(time.RFC3339Nano, str(run["finishedAt"]))
		check(e == nil, "invalid run finish timestamp")
		wall = f.Sub(s).Seconds()
	}
	billing := obj(response["cloudBilling"])
	generations := []any{}
	for _, i := range rows(billing["instances"]) {
		generations = append(generations, Object{"instance_id": i["instanceId"], "status": i["status"], "billing": pick(obj(i["billing"]), "pricingStatus", "priceCurrency", "priceSource", "priceObservedAt", "priceRegion", "priceMachineType", "startedAt", "finishedAt", "computeStartedAt", "accumulatedComputeSeconds", "computePricePerSecond", "diskPricePerSecond", "asOf", "currentCost", "missingPrices")})
	}
	costLimitation := "Cloud prices are incomplete; cost zero does not mean free cloud execution."
	if billing["status"] == "estimated" {
		costLimitation = "Cloud lifecycle cost is a list-rate estimate with explicit exclusions, not the provider invoice. The task/window API cost and plan retention-window cost have different bases."
	}
	return Object{"schema_version": 1, "run_id": run["id"], "attempt_history": history, "audit": reportAudit, "plan": metrics(obj(get(obj(request["plan"]), "predicted"))), "plan_assignment_compute_work_seconds": sum(rows(fileValues(assignments)), "predictedRuntimeSeconds"), "plan_assignment_transfer_work_seconds": sum(rows(fileValues(assignments)), "predictedTransferSeconds"), "simulation": metrics(obj(get(simulation, "run"))), "real_api_reported": metrics(run), "real_cloud_lifecycle_estimate": pick(billing, "status", "currency", "estimatedCost", "knownCost", "costBasis", "excludedCosts", "asOf"), "real_cloud_generations": generations, "real_api_run_timestamp_wall_seconds": wall, "real_network_bytes_from_routes": network, "transfer_route_count": len(transfers), "dependency_route_count": len(deps), "cross_cloud_hpc_relay_count": cross, "relay_server_network_bytes": relay, "routes": routeRows, "shared_preparations": shared, "activities": activityRows, "limitations": list(attemptLimitation, "Runtime finish semantics depend on the driver; observation delay is exported only for exact-attempt handles with validated terminal timestamps.", "API aggregate counters can contain older attempts; audit uses latest distinct IDs.", "Transfer work is summed overlapping route time, not wall-clock preparation.", "Plan assignment work sums are not makespan; API timestamp wall and recovered makespan can differ.", costLimitation, "e2-medium interference extrapolated to HPC/micro; micro speedup 0.25 is approximate.", "Whole-workspace real traffic differs from estimated plan edge volumes.", "Zero reported interference does not prove absence of physical contention.")}
}
