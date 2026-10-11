package montagetools

import (
	"encoding/json"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
	"math"
	"sort"
	"time"
)

// Calibration accepts only the training reference. Holdout data never enter
// this function. Fixed placement and compute profiles remain unchanged.
func calibrateHPCStages(request, training Object, sha string) Object {
	return calibrateRuntimeStages(request, training, sha, "hpc")
}

func calibrateRuntimeStages(request, training Object, sha, targetClass string) Object {
	check(targetClass != "", "target resource class required")
	request = clone(request)
	check(obj(training["run"])["status"] == "completed", "completed training reference required")
	var typed ports.ExecutionRequest
	check(json.Unmarshal(encode(request), &typed) == nil, "invalid request")
	types := map[string]string{}
	for _, a := range typed.Plan.Assignments {
		for _, r := range typed.Resources {
			if r.ID == a.ResourceID {
				types[a.ActivityID] = string(r.Type)
			}
		}
	}
	projected, err := workflow.ProjectWorkspaceDependenciesForResourceTypes(typed.Workflow, types)
	check(err == nil, "projection failed")
	vol := map[string]workflow.WorkspaceProjectionVolumes{}
	for _, a := range projected.Activities {
		var v workflow.WorkspaceProjectionVolumes
		b, _ := json.Marshal(a.Metadata["workspaceProjectionVolumes"])
		check(json.Unmarshal(b, &v) == nil, "invalid projected volume")
		vol[a.ID] = v
	}
	handles := map[string][]Object{}
	for _, h := range rows(training["handles"]) {
		handles[str(h["activityId"])] = append(handles[str(h["activityId"])], h)
	}
	completed := map[string]float64{}
	for _, e := range rows(training["events"]) {
		if e["eventType"] == "activity.completed" {
			at, err := time.Parse(time.RFC3339Nano, str(e["occurredAt"]))
			check(err == nil, "invalid event timestamp")
			completed[str(e["aggregateId"])] = float64(at.UnixNano()) / 1e9
		}
	}
	merge := map[string]Object{}
	gateway := map[string]bool{}
	for _, t := range rows(training["artifactTransferRuns"]) {
		if t["status"] != "completed" {
			continue
		}
		id := str(t["activityId"])
		if t["strategy"] == "gateway" {
			gateway[id] = true
		}
		r := obj(t["route"])
		if (t["strategy"] == "direct-runtime" || t["strategy"] == "runtime-local") && r["sharedFilesystem"] != nil && r["sharedFilesystem"] != "" && r["mergeCheckSeconds"] != nil {
			if merge[id] == nil || num(t["startedAt"]) > num(merge[id]["at"]) {
				merge[id] = Object{"at": t["startedAt"], "route": r}
			}
		}
	}
	var checkTime, copyTime, verifyTime, totalBytes, uniqueBytes, commandTime float64
	mergeN := 0
	prepX, prepY, pubX, pubY := []float64{}, []float64{}, []float64{}, []float64{}
	overhead := []float64{}
	runtimeIDs := map[string]bool{}
	for _, t := range rows(training["activities"]) {
		id := str(t["activityId"])
		hs := handles[id]
		if t["status"] != "completed" || len(hs) != 1 || hs[0]["status"] != "completed" || math.Abs(num(t["startedAt"])-num(hs[0]["startedAt"])) > .001 {
			continue
		}
		m := obj(t["metadata"])
		if resourceClass(str(t["allocatedResourceId"])) != targetClass {
			continue
		}
		runtimeIDs[str(t["runtimeId"])] = true
		v := vol[id]
		overhead = append(overhead, num(m["launchWaitSeconds"])+num(t["overheadSeconds"]))
		if finish, ok := completed[str(t["id"])]; ok && finish >= num(t["finishedAt"]) {
			pubX = append(pubX, float64(v.PreExecutionBytes+v.CreatedOutputBytes))
			pubY = append(pubY, finish-num(t["finishedAt"]))
		}
		if row := merge[id]; row != nil && !gateway[id] {
			r := obj(row["route"])
			checkTime += num(r["mergeCheckSeconds"])
			copyTime += num(r["mergeCopySeconds"])
			verifyTime += num(r["mergeVerifySeconds"])
			commandTime += num(r["mergeCommandOverheadSeconds"])
			totalBytes += float64(v.InputTotalBytes)
			uniqueBytes += float64(v.InputUniqueBytes)
			mergeN++
			residual := num(m["transferElapsedSeconds"]) - num(r["mergeExecutionSeconds"]) - num(r["mergeSshChannelWaitSeconds"])
			prepX = append(prepX, float64(v.InputUniqueBytes))
			prepY = append(prepY, math.Max(0, residual))
		}
	}
	check(mergeN > 0 && uniqueBytes > 0 && len(pubX) > 0 && len(overhead) > 0, "insufficient single-attempt runtime evidence")
	check(len(runtimeIDs) == 1, "cannot mix runtime drivers in one class calibration")
	prepFixed, prepRate := positiveLine(prepX, prepY)
	pubFixed, pubRate := positiveLine(pubX, pubY)
	sort.Float64s(overhead)
	fixedOverhead := overhead[len(overhead)/2]
	model := obj(obj(obj(request["NetworkTopology"])["metadata"])["workspaceMaterializationModel"])
	replace := func(key string, profile Object) {
		p := []any{}
		for _, old := range rows(model[key]) {
			if old["targetClass"] != targetClass {
				p = append(p, old)
			}
		}
		model[key] = append(p, profile)
	}
	source := "runtime-stage-training:" + sha
	replace("mergeProfiles", Object{"targetClass": targetClass, "source": source, "samples": mergeN, "commandOverheadSeconds": commandTime / float64(mergeN), "checkSecondsPerInputByte": checkTime / totalBytes, "copySecondsPerUniqueByte": copyTime / uniqueBytes, "verifySecondsPerUniqueByte": verifyTime / uniqueBytes})
	replace("inputObservationProfiles", Object{"targetClass": targetClass, "source": source + ":effective-preparation-residual-not-queue-free", "samples": len(prepX), "fixedSeconds": prepFixed, "secondsPerUniqueByte": prepRate})
	replace("publicationProfiles", Object{"targetClass": targetClass, "source": source + ":publication-and-poll-wall", "samples": len(pubX), "fixedSeconds": pubFixed, "scanSecondsPerWorkspaceByte": pubRate, "cleanupSecondsPerInputByte": 0})
	for _, r := range rows(request["Resources"]) {
		if r["schedulable"] != true {
			continue
		}
		if resourceClass(str(r["id"])) == targetClass {
			r["containerOverheadSeconds"] = fixedOverhead
		}
	}
	for _, a := range rows(obj(request["Plan"])["assignments"]) {
		if resourceClass(str(a["resourceId"])) != targetClass {
			continue
		}
		m := obj(a["metadata"])
		m["containerOverheadSeconds"] = fixedOverhead
	}
	obj(obj(request["NetworkTopology"])["metadata"])["runtimeStageCalibration"] = Object{"issue": "AKO-165", "targetClass": targetClass, "runtimeIds": runtimeIDs, "trainingRunId": obj(training["run"])["id"], "trainingSHA256": sha, "mergeSamples": mergeN, "preparationResidualSamples": len(prepX), "publicationSamples": len(pubX), "fixedLaunchAndRuntimeOverheadSeconds": fixedOverhead, "holdoutUsed": false, "limitations": list("Recovered/multi-handle training activities excluded.", "Preparation residual and publication contain unidentified waiting/poll delay; exploratory effective-wall fit, not queue-free service.", "Scheduler wait and gateway fallback not injected into the internal-route baseline; evaluate residuals separately.", "Fixed overhead median does not capture family-specific wrapper cost.", "Classes without same-storage evidence are rejected, not borrowed from HPC; per-route network service remains independently calibrated.")}
	return request
}

func positiveLine(xs, ys []float64) (float64, float64) {
	var sx, sy, sxx, sxy float64
	for i, x := range xs {
		x /= 1e9
		sx += x
		sy += ys[i]
		sxx += x * x
		sxy += x * ys[i]
	}
	n := float64(len(xs))
	candidates := [][2]float64{{sy / n, 0}}
	if sxx > 0 {
		candidates = append(candidates, [2]float64{0, sxy / sxx})
	}
	if d := n*sxx - sx*sx; d > 1e-12 {
		b := (n*sxy - sx*sy) / d
		a := (sy - b*sx) / n
		if a >= 0 && b >= 0 {
			candidates = append(candidates, [2]float64{a, b})
		}
	}
	best := candidates[0]
	score := math.Inf(1)
	for _, c := range candidates {
		var s float64
		for i, x := range xs {
			e := c[0] + c[1]*x/1e9 - ys[i]
			s += e * e
		}
		if s < score {
			score = s
			best = c
		}
	}
	return best[0], best[1] / 1e9
}
