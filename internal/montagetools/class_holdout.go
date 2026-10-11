package montagetools

import (
	"encoding/json"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
	"sort"
)

// Prepare a private simulation-only case with one VM and one execution lane.
// No API call, provisioning, real execution, or runtime catalog mutation.
func prepareClassHoldout(template, calibration Object, target, runID, planID string) Object {
	check(runPattern.MatchString(runID) && runPattern.MatchString(planID), "invalid holdout identity")
	var typed ports.ExecutionRequest
	check(json.Unmarshal(encode(template), &typed) == nil, "invalid template")
	out := Object{}
	check(json.Unmarshal(encode(typed), &out) == nil, "cannot normalize template")
	var resource Object
	for _, r := range rows(out["Resources"]) {
		if r["id"] == target {
			resource = r
		}
	}
	check(resource != nil && resource["type"] == "cloud_vm" && resource["schedulable"] == true, "eligible existing cloud target required")
	metadata := obj(resource["metadata"])
	metadata["maximumConcurrentActivities"] = 1
	resource["metadata"] = metadata
	class := resourceClass(target)
	model := obj(obj(obj(out["NetworkTopology"])["metadata"])["workspaceMaterializationModel"])
	model["schemaVersion"] = 2
	calModel := obj(obj(obj(calibration["NetworkTopology"])["metadata"])["workspaceMaterializationModel"])
	for _, key := range []string{"mergeProfiles", "inputObservationProfiles", "publicationProfiles"} {
		profiles := []any{}
		found := false
		for _, p := range rows(model[key]) {
			if p["targetClass"] != class {
				profiles = append(profiles, p)
			}
		}
		for _, p := range rows(calModel[key]) {
			if p["targetClass"] == class {
				check(num(p["samples"]) > 0 && p["source"] != nil, "profile evidence required")
				profiles = append(profiles, p)
				found = true
			}
		}
		check(found, "missing calibrated stage for class "+class)
		model[key] = profiles
	}
	provenance := obj(obj(obj(calibration["NetworkTopology"])["metadata"])["runtimeStageCalibration"])
	check(provenance["targetClass"] == class, "calibration class mismatch")
	fixed := num(provenance["fixedLaunchAndRuntimeOverheadSeconds"])
	resource["containerOverheadSeconds"] = fixed
	profiles, err := workflow.CompleteSpeedupProfiles(typed.Workflow.Activities, typed.Resources, typed.ActivityProfiles)
	check(err == nil, "cannot complete compute estimates")
	out["ActivityProfiles"] = profiles
	nodes := map[string]bool{}
	indegree := map[string]int{}
	successors := map[string][]string{}
	edges := map[string]bool{}
	for _, a := range typed.Workflow.Activities {
		nodes[a.ID] = true
		indegree[a.ID] = 0
	}
	add := func(parent, child string) {
		check(nodes[parent] && nodes[child], "unknown dependency")
		key := parent + "\x00" + child
		if !edges[key] {
			edges[key] = true
			indegree[child]++
			successors[parent] = append(successors[parent], child)
		}
	}
	for _, d := range typed.Workflow.Dependencies {
		add(d.DependsOnActivityID, d.ActivityID)
	}
	for _, d := range typed.Workflow.DataDependencies {
		add(d.ProducerActivityID, d.ConsumerActivityID)
	}
	ordered := []string{}
	ready := []string{}
	for id, n := range indegree {
		if n == 0 {
			ready = append(ready, id)
		}
	}
	for len(ready) > 0 {
		sort.Strings(ready)
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, id)
		for _, child := range successors[id] {
			indegree[child]--
			if indegree[child] == 0 {
				ready = append(ready, child)
			}
		}
	}
	check(len(ordered) == len(nodes), "cyclic workflow cannot be serialized")
	assignments := []any{}
	for i, id := range ordered {
		assignments = append(assignments, Object{"id": planID + "-" + id, "planId": planID, "activityId": id, "resourceId": target, "coreId": target + "-core-1", "orderOnResource": i, "metadata": Object{"containerOverheadSeconds": fixed}})
	}
	plan := obj(out["Plan"])
	plan["id"] = planID
	plan["assignments"] = assignments
	plan["lifecycleActions"] = []any{}
	plan["source"] = "manual"
	plan["algorithm"] = "fixed-class-holdout"
	plan["metadata"] = Object{"reviewOnly": true, "realExecutionAuthorized": false, "experimentRole": "independent-class-holdout-preparation-only", "targetClass": class, "oneVmOneExecutionLane": true, "trainingEvidence": provenance, "priceStatus": "historical-estimate-revalidate-before-real"}
	plan["metadata"].(map[string]any)["remainingRealPreflight"] = list("User approval required.", "Register/freeze lifecycle actions and include provisioning/disk costs before real.", "Revalidate quota, SSH, runtime and physical disk availability.")
	out["Run"] = Object{"id": runID, "schedulePlanId": planID, "mode": "simulation", "seed": 1}
	out["RuntimeAllocations"] = Object{}
	return out
}
