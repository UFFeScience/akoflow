package montagetools

import "strings"

// Exploratory effective route wall profiles, not queue-free service rates.
// Shared consumer merge/barrier phases are not charged per dependency.
func transferWallSensitivity(request, real Object, provenance string) Object {
	request = clone(request)
	check(obj(real["run"])["status"] == "completed", "completed transfer reference required")
	latest := map[string]Object{}
	workspaceBytes := map[string]float64{}
	for _, workspace := range rows(real["workspaces"]) {
		id := str(workspace["activityId"])
		check(workspaceBytes[id] == 0, "ambiguous source workspace volume")
		workspaceBytes[id] = num(workspace["sizeBytes"])
	}
	for _, transfer := range rows(real["artifactTransferRuns"]) {
		route := obj(transfer["route"])
		if transfer["status"] != "completed" || route["producerActivityId"] == nil || route["consumerActivityId"] == nil {
			continue
		}
		key := str(route["producerActivityId"]) + "/" + str(route["consumerActivityId"])
		if latest[key] == nil || num(transfer["startedAt"]) > num(latest[key]["startedAt"]) {
			latest[key] = transfer
		}
	}
	groups := map[string]Object{}
	for _, transfer := range latest {
		strategy := transfer["strategy"]
		if strategy != "direct-runtime" && strategy != "gateway" {
			continue
		}
		route := obj(transfer["route"])
		// logicalBytes can contain only newly produced outputs (e.g. a tiny
		// table), while the archive carries all inherited workspace files.
		workspace, archive := workspaceBytes[str(route["producerActivityId"])], num(route["archiveBytes"])
		if workspace <= 0 || archive <= 0 {
			continue
		}
		from, to := resourceClass(str(route["sourceResourceId"])), resourceClass(str(route["targetResourceId"]))
		key := str(strategy) + "/" + from + "/" + to
		phaseSeconds := map[string]float64{}
		for _, phase := range rows(route["phases"]) {
			name := str(phase["name"])
			if strings.HasPrefix(name, "shared-") {
				continue
			}
			seconds := num(phase["durationSeconds"]) - num(phase["sshChannelWaitSeconds"])
			check(seconds >= 0, "negative transfer phase service after observed admission subtraction")
			phaseSeconds[name] += seconds
		}
		if phaseSeconds["peer-private-upload"] <= 0 && phaseSeconds["archive-upload"] <= 0 {
			continue
		}
		g := groups[key]
		if g == nil {
			g = Object{"strategy": strategy, "sourceClass": from, "targetClass": to}
			groups[key] = g
		}
		for name, value := range map[string]float64{"workspace": workspace, "archive": archive, "samples": 1, "probe": phaseSeconds["peer-authenticated-connectivity-probe"], "snapshot": phaseSeconds["peer-snapshot-acquire"], "upload": phaseSeconds["peer-private-upload"] + phaseSeconds["archive-upload"], "receiver": phaseSeconds["peer-receiver-verify-and-extract"] + phaseSeconds["receiver-verify-and-extract"], "ingress": phaseSeconds["pack-and-receive"], "validation": phaseSeconds["archive-validation"]} {
			g[name] = num(g[name]) + value
		}
	}
	metadata := obj(obj(request["NetworkTopology"])["metadata"])
	model := obj(metadata["workspaceMaterializationModel"])
	updated := []any{}
	for _, profile := range rows(model["profiles"]) {
		key := str(profile["strategy"]) + "/" + str(profile["sourceClass"]) + "/" + str(profile["targetClass"])
		g := groups[key]
		if g == nil {
			continue
		}
		profile["archiveToWorkspaceRatio"] = num(g["archive"]) / num(g["workspace"])
		profile["uploadArchiveBytesPerSecond"] = num(g["archive"]) / num(g["upload"])
		profile["receiverSecondsPerByte"] = num(g["receiver"]) / num(g["workspace"])
		profile["source"], profile["samples"] = "exploratory-effective-transfer-wall:"+provenance, int(num(g["samples"]))
		if profile["strategy"] == "direct-runtime" {
			profile["probeSeconds"] = num(g["probe"]) / num(g["samples"])
			profile["snapshotSecondsPerByte"] = num(g["snapshot"]) / num(g["workspace"])
		} else {
			check(num(g["ingress"]) > 0, "gateway ingress phase missing")
			profile["ingressArchiveBytesPerSecond"] = num(g["archive"]) / num(g["ingress"])
			profile["validationSecondsPerByte"] = num(g["validation"]) / num(g["workspace"])
		}
		updated = append(updated, clone(profile))
	}
	check(len(updated) > 0, "no comparable measured route profiles")
	// Wall samples may contain unobserved admission; do not put another SSH
	// semaphore queue on top of their effective durations in this sensitivity.
	for _, value := range obj(model["resources"]) {
		resource := obj(value)
		delete(resource, "bulkServiceGroupId")
		delete(resource, "bulkServiceSlots")
		delete(resource, "bulkServiceSource")
	}
	metadata["transferWallSensitivity"] = Object{"referenceSHA256": provenance, "profiles": updated, "role": "effective-wall-route-sensitivity-not-independent-validation", "limitations": list("Unknown SSH/cache waiting and CPU/IO contention remain in effective wall coefficients.", "Explicit SSH admission groups are disabled for this offline sensitivity to avoid adding another queue on wall-fit samples.", "Network sharing remains simulated; upload wall samples may already contain sharing, so double-counting risk is not eliminated.", "Shared merge is retained once per consumer, not replicated per predecessor.", "No real execution is authorized or submitted.")}
	return request
}
