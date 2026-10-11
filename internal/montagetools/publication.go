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
	cleanup := map[string]float64{}
	for _, workspace := range rows(real["workspaces"]) {
		// Released intermediate workspaces are reclaimed later, after their
		// consumers; that is not publication cleanup of the producer.
		if workspace["isFinal"] == true && workspace["retention"] == "final" {
			cleanup[str(get(workspace, "activityId"))] = num(workspace["reclaimedBytes"])
		}
	}
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
		observations = append(observations, Object{"activityId": task["activityId"], "targetClass": resourceClass(str(get(handle, "resourceId"))), "terminalScanBytes": scanBytes, "publicationCleanupBytes": cleanup[str(task["activityId"])], "publicationWallSeconds": wall, "pollDelayExcludedSeconds": observed - num(get(handle, "finishedAt"))})
	}
	check(len(observations) > 0, "no publication observations")
	return Object{"runId": obj(real["run"])["id"], "referenceSHA256": referenceDigest, "role": "publication-wall-reference-not-queue-free-service-fit", "samples": len(observations), "weightedWallSecondsPerScanByte": totalSeconds / totalBytes, "summedWorkSecondsNotMakespan": totalSeconds, "observations": observations, "limitations": list("Includes observation SSH transport, sealing, pruning and scheduling between terminal observation and completion event.", "Excludes initial polling delay but does not prove zero admission wait.", "Recovered reference is not independent validation; no global makespan multiplier is applied.")}
}

func combinedObservationSensitivity(request, real Object, digest string) Object {
	request = observationSensitivity(request, real, digest)
	reference := publicationReference(real, digest)
	volumes := map[string]float64{}
	cleanup := map[string]float64{}
	for _, row := range rows(reference["observations"]) {
		volumes[str(get(row, "activityId"))] = num(get(row, "terminalScanBytes"))
		cleanup[str(get(row, "activityId"))] = num(row["publicationCleanupBytes"])
	}
	for _, activity := range rows(get(obj(get(request, "Workflow")), "activities")) {
		bytes, ok := volumes[str(get(activity, "id"))]
		if !ok {
			continue
		}
		projection := obj(get(obj(get(activity, "metadata")), "workspaceProjection"))
		projection["terminalObservationBytes"] = int64(bytes)
		projection["publicationCleanupBytes"] = int64(cleanup[str(activity["id"])])
		projection["terminalObservationSource"] = "observed-pre-prune-snapshot:" + digest
	}
	metadata := obj(get(obj(get(request, "NetworkTopology")), "metadata"))
	model := obj(get(metadata, "workspaceMaterializationModel"))
	scan, prune := fitPublicationWall(rows(reference["observations"]))
	model["publicationProfiles"] = []any{Object{"targetClass": "e2-micro", "source": "combined-observed-publication-wall:" + digest, "samples": reference["samples"], "scanSecondsPerWorkspaceByte": scan, "cleanupSecondsPerInputByte": prune}}
	metadata["publicationObservationSensitivity"] = reference
	return request
}

// Two-feature nonnegative least squares; no total-makespan scaling. Features
// are normalized to GB for numeric stability. The fitted wall coefficients
// remain exploratory until transport/admission wait is independently measured.
func fitPublicationWall(rows []Object) (float64, float64) {
	var xx, xz, zz, xy, zy float64
	for _, row := range rows {
		x := num(row["terminalScanBytes"]) / 1e9
		z := num(row["publicationCleanupBytes"]) / 1e9
		y := num(row["publicationWallSeconds"])
		xx += x * x
		xz += x * z
		zz += z * z
		xy += x * y
		zy += z * y
	}
	candidates := [][2]float64{{0, 0}}
	if xx > 0 {
		candidates = append(candidates, [2]float64{xy / xx, 0})
	}
	if zz > 0 {
		candidates = append(candidates, [2]float64{0, zy / zz})
	}
	if determinant := xx*zz - xz*xz; determinant > 1e-12 {
		a, b := (xy*zz-zy*xz)/determinant, (zy*xx-xy*xz)/determinant
		if a >= 0 && b >= 0 {
			candidates = append(candidates, [2]float64{a, b})
		}
	}
	best, errorSquared := candidates[0], math.Inf(1)
	for _, candidate := range candidates {
		var residual float64
		for _, row := range rows {
			predicted := candidate[0]*num(row["terminalScanBytes"])/1e9 + candidate[1]*num(row["publicationCleanupBytes"])/1e9
			delta := predicted - num(row["publicationWallSeconds"])
			residual += delta * delta
		}
		if residual < errorSquared {
			best, errorSquared = candidate, residual
		}
	}
	return best[0] / 1e9, best[1] / 1e9
}
