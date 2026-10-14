package montagetools

import (
	"regexp"
	"strings"
)

var checksumPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func files(snapshot any) Object {
	result := Object{}
	for _, f := range rows(snapshot) {
		name := str(get(f, "path"))
		check(!strings.HasPrefix(name, "/") && !strings.Contains(name, "\x00") && !strings.Contains(name, "\\"), "Unsafe scientific manifest path")
		for _, p := range strings.Split(name, "/") {
			check(p != "..", "Unsafe scientific manifest path")
		}
		check(checksumPattern.MatchString(str(get(f, "checksum"))), "Missing content identity")
		check(num(get(f, "sizeBytes")) >= 0, "Negative manifest size")
		check(result[name] == nil, "Duplicate manifest path")
		result[name] = Object{"path": name, "sizeBytes": f["sizeBytes"], "contentId": f["checksum"]}
	}
	return result
}
func fileValues(m Object) []any {
	out := []any{}
	for _, k := range sortedKeys(m) {
		out = append(out, m[k])
	}
	return out
}
func fileBytes(m Object) float64 { return sum(rows(fileValues(m)), "sizeBytes") }
func unionFiles(sources []Object) Object {
	u := Object{}
	for _, source := range sources {
		for _, path := range sortedKeys(source) {
			f := source[path]
			check(u[path] == nil || eq(u[path], f), "Baseline parent content conflict")
			u[path] = f
		}
	}
	return u
}
func activityModel(inherited, final, packaging Object, source string) Object {
	outputs := []any{}
	removed := []any{}
	for _, p := range sortedKeys(final) {
		f := final[p]
		if !eq(inherited[p], f) && !eq(packaging[p], f) {
			outputs = append(outputs, f)
		}
	}
	for _, p := range sortedKeys(inherited) {
		if final[p] == nil {
			removed = append(removed, p)
		}
	}
	inputs := fileValues(packaging)
	return Object{"contract": "whole-workspace-v1", "source": source, "inputs": inputs, "inputsByResourceType": Object{"cloud_vm": inputs, "hpc_machine": []any{}}, "outputs": outputs, "removedPaths": removed}
}
func measuredVolumes(audit, raw, request Object, d string) Object {
	validateBaseline(audit, d, nil)
	check(obj(get(raw, "run"))["id"] == audit["run_id"] && obj(raw["run"])["status"] == "completed", "Completed reference identity mismatch")
	snapshots := Object{}
	for _, h := range rows(get(raw, "handles")) {
		check(h["status"] == "completed" && h["exitCode"] != nil && num(h["exitCode"]) == 0, "Unsuccessful reference handle")
		id := str(get(h, "activityId"))
		check(snapshots[id] == nil, "Duplicate reference handle")
		snapshots[id] = files(get(obj(get(h, "artifacts")), "finalSnapshot"))
	}
	check(len(snapshots) == 58, "Reference snapshots do not match activities")
	for _, a := range rows(audit["activities"]) {
		check(snapshots[str(a["activity_id"])] != nil, "Reference snapshots do not match activities")
	}
	inputs := Object{}
	parents := Object{}
	for _, e := range rows(get(obj(get(request, "workflow")), "dependencies")) {
		id := str(e["activityId"])
		parents[id] = append(arr(parents[id]), get(e, "dependsOnActivityId"))
	}
	bytes := Object{}
	for _, id := range sortedKeys(snapshots) {
		sources := []Object{}
		total := 0.0
		for _, p := range arr(parents[id]) {
			m := obj(get(snapshots, str(p)))
			sources = append(sources, m)
			total += fileBytes(m)
		}
		u := unionFiles(sources)
		inputs[id] = Object{"inputTotalBytes": total, "inputUniqueBytes": fileBytes(u), "inputFiles": len(u)}
		bytes[id] = fileBytes(obj(snapshots[id]))
	}
	durations := Object{}
	for _, a := range rows(get(obj(request["workflow"]), "activities")) {
		durations[str(a["id"])] = get(obj(get(a, "simulation")), "durationSeconds")
	}
	return Object{"baselineRunId": audit["run_id"], "auditedEvidenceSHA256": d, "activityReferenceDurations": durations, "observedProducerWorkspaceBytes": bytes, "observedConsumerInputVolumes": inputs}
}
func projection(audit, raw, request Object, d string) Object {
	result := measuredVolumes(audit, raw, request, d)
	workflow := obj(request["workflow"])
	ids := []string{}
	for _, a := range rows(workflow["activities"]) {
		ids = append(ids, str(a["id"]))
	}
	check(len(ids) == 58, "Baseline handle mismatch")
	snapshots := Object{}
	for _, h := range rows(raw["handles"]) {
		snapshots[str(h["activityId"])] = files(obj(h["artifacts"])["finalSnapshot"])
	}
	preds := Object{}
	for _, e := range rows(workflow["dependencies"]) {
		id := str(e["activityId"])
		preds[id] = append(arr(preds[id]), e["dependsOnActivityId"])
	}
	resources := index(request["resources"], "id")
	assignments := Object{}
	for _, a := range rows(obj(request["plan"])["assignments"]) {
		assignments[str(a["activityId"])] = get(obj(get(resources, str(a["resourceId"]))), "type")
	}
	cloudRoots, hpcRoots := []string{}, []string{}
	for _, id := range ids {
		if len(arr(preds[id])) == 0 {
			if assignments[id] == "cloud_vm" {
				cloudRoots = append(cloudRoots, id)
			}
			if assignments[id] == "hpc_machine" {
				hpcRoots = append(hpcRoots, id)
			}
		}
	}
	check(len(cloudRoots) >= 2, "Need multiple OCI root observations")
	common := clone(obj(snapshots[cloudRoots[0]]))
	for _, id := range cloudRoots[1:] {
		for _, p := range sortedKeys(common) {
			if !eq(common[p], obj(snapshots[id])[p]) {
				delete(common, p)
			}
		}
	}
	check(len(common) > 0, "Packaging evidence missing")
	check(len(hpcRoots) > 0, "HPC packaging evidence missing")
	for _, id := range hpcRoots {
		for _, p := range sortedKeys(common) {
			check(obj(snapshots[id])[p] == nil, "OCI packaging unexpectedly present on HPC root")
		}
	}
	models := Object{}
	for _, id := range ids {
		sources := []Object{}
		for _, parent := range arr(preds[id]) {
			sources = append(sources, obj(get(snapshots, str(parent))))
		}
		models[id] = activityModel(unionFiles(sources), obj(get(snapshots, id)), common, "baseline:"+str(audit["run_id"]))
	}
	result["schemaVersion"] = 1
	result["contract"] = "whole-workspace-v1"
	result["runtimePackagingFiles"] = len(common)
	result["activityModels"] = models
	result["dependencies"] = workflow["dependencies"]
	result["baselineAssignedResourceTypes"] = assignments
	result["limitations"] = list(
		"File sizes and identities are previous-run observations for the same immutable image and dataset, not future observations.",
		"Runtime packaging variant is supported only for evidenced cloud_vm and hpc_machine types.",
		"No transfer deduplication, snapshot reuse or shared storage is inferred by this manifest model.",
	)
	return result
}
