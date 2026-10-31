package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// physicalSignature intentionally excludes scheduler labels and cost forecasts,
// but retains the complete frozen workflow, infrastructure, profiles, topology,
// lane order and predicted timing. Equal signatures authorize reuse of one
// observation, not a claim of independent experimental replicates.
func physicalSignature(request, plan obj) (string, error) {
	input := clone(request)
	for _, k := range []string{"Run", "run", "Plan", "plan", "Recovery", "recovery", "PreparationRequirementsByActivity", "RuntimeAllocations"} {
		delete(input, k)
	}
	resources := map[string]obj{}
	rawResources, _ := input["Resources"].([]any)
	for _, raw := range rawResources {
		r := raw.(map[string]any)
		resources[fmt.Sprint(r["id"])] = r
		if m, ok := r["metadata"].(map[string]any); ok {
			delete(m, "workspaceStorageQuotaOverride")
			delete(m, "workspaceStorageBudget")
		}
	}
	assignments := []obj{}
	items, _ := plan["assignments"].([]any)
	for _, raw := range items {
		a := raw.(map[string]any)
		r := resources[fmt.Sprint(a["resourceId"])]
		m, _ := a["metadata"].(map[string]any)
		v := obj{}
		for _, k := range []string{"activityId", "resourceId", "coreId", "orderOnResource", "priority", "predictedReadyAt", "predictedStartAt", "predictedFinishAt", "predictedRuntimeSeconds", "predictedTransferSeconds"} {
			if a[k] != nil {
				v[k] = a[k]
			}
		}
		value := func(k string, fallback any) any {
			if x, ok := m[k]; ok {
				return x
			}
			if fallback != nil {
				return fallback
			}
			return 0.
		}
		v["bootOverheadSeconds"] = value("bootOverheadSeconds", r["bootOverheadSeconds"])
		v["resourceAvailableAtSeconds"] = value("resourceAvailableAtSeconds", 0.)
		v["containerOverheadSeconds"] = value("containerOverheadSeconds", r["containerOverheadSeconds"])
		v["expectedDurationSeconds"] = value("expectedDurationSeconds", a["predictedRuntimeSeconds"])
		for k, x := range v {
			if n, ok := x.(float64); ok {
				v[k] = math.Round(n*1e9) / 1e9
			}
		}
		assignments = append(assignments, v)
	}
	sort.Slice(assignments, func(i, j int) bool {
		return fmt.Sprint(assignments[i]["activityId"]) < fmt.Sprint(assignments[j]["activityId"])
	})
	b, e := json.Marshal(obj{"inputs": input, "assignments": assignments, "seed": 1})
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum), nil
}
