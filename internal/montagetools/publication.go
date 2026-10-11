package montagetools

import (
	"math"
	"time"
)

func publicationReference(real Object, referenceDigest string) Object {
	check(obj(get(real, "run"))["status"] == "completed", "completed reference required")
	completed := map[string]float64{}
	for _, event := range rows(get(real, "events")) {
		if event["eventType"] != "activity.completed" {
			continue
		}
		timestamp, err := time.Parse(time.RFC3339Nano, str(get(event, "occurredAt")))
		check(err == nil, "invalid completion timestamp")
		completed[str(get(event, "aggregateId"))] = float64(timestamp.UnixNano()) / 1e9
	}
	latest := map[string]Object{}
	for _, handle := range rows(get(real, "handles")) {
		id := str(get(handle, "activityId"))
		if latest[id] == nil || num(handle["startedAt"]) > num(latest[id]["startedAt"]) {
			latest[id] = handle
		}
	}
	observations := []any{}
	var totalSeconds, totalBytes float64
	for _, task := range rows(get(real, "activities")) {
		handle := latest[str(get(task, "activityId"))]
		if task["status"] != "completed" || handle == nil || handle["status"] != "completed" {
			continue
		}
		metadata := obj(handle["metadata"])
		if metadata["executionTarget"] != "remote-docker" {
			continue
		}
		check(math.Abs(num(task["startedAt"])-num(handle["startedAt"])) < .001, "latest task/handle mismatch")
		at, ok := completed[str(get(task, "id"))]
		check(ok, "completed publication event missing")
		observed := num(get(metadata, "terminalObservedAt"))
		check(at >= observed, "publication precedes terminal observation")
		var scanBytes float64
		for _, file := range rows(get(obj(get(handle, "artifacts")), "finalSnapshot")) {
			scanBytes += num(get(file, "sizeBytes"))
		}
		check(scanBytes > 0, "terminal snapshot volume missing")
		wall := at - observed
		totalSeconds += wall
		totalBytes += scanBytes
		observations = append(observations, Object{"activityId": task["activityId"], "targetClass": resourceClass(str(get(handle, "resourceId"))), "terminalScanBytes": scanBytes, "publicationWallSeconds": wall, "pollDelayExcludedSeconds": observed - num(get(handle, "finishedAt"))})
	}
	check(len(observations) > 0, "no publication observations")
	return Object{"runId": obj(real["run"])["id"], "referenceSHA256": referenceDigest, "role": "publication-wall-reference-not-queue-free-service-fit", "samples": len(observations), "weightedWallSecondsPerScanByte": totalSeconds / totalBytes, "summedWorkSecondsNotMakespan": totalSeconds, "observations": observations, "limitations": list("Includes observation SSH transport, sealing, pruning and scheduling between terminal observation and completion event.", "Excludes initial polling delay but does not prove zero admission wait.", "Recovered reference is not independent validation; no global makespan multiplier is applied.")}
}

func combinedObservationSensitivity(request, real Object, digest string) Object {
	request = observationSensitivity(request, real, digest)
	reference := publicationReference(real, digest)
	volumes := map[string]float64{}
	for _, row := range rows(reference["observations"]) {
		volumes[str(get(row, "activityId"))] = num(get(row, "terminalScanBytes"))
	}
	for _, activity := range rows(get(obj(get(request, "Workflow")), "activities")) {
		bytes, ok := volumes[str(get(activity, "id"))]
		if !ok {
			continue
		}
		projection := obj(get(obj(get(activity, "metadata")), "workspaceProjection"))
		projection["terminalObservationBytes"] = int64(bytes)
		projection["terminalObservationSource"] = "observed-pre-prune-snapshot:" + digest
	}
	metadata := obj(get(obj(get(request, "NetworkTopology")), "metadata"))
	model := obj(get(metadata, "workspaceMaterializationModel"))
	model["publicationProfiles"] = []any{Object{"targetClass": "e2-micro", "source": "combined-observed-publication-wall:" + digest, "samples": reference["samples"], "scanSecondsPerWorkspaceByte": reference["weightedWallSecondsPerScanByte"], "cleanupSecondsPerInputByte": 0}}
	metadata["publicationObservationSensitivity"] = reference
	return request
}
