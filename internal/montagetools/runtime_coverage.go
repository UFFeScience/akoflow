package montagetools

import (
	"sort"
	"strings"
)

func runtimeReferenceCoverage(references []Object) Object {
	result := []any{}
	for _, reference := range references {
		run := obj(reference["run"])
		handles := map[string][]Object{}
		for _, h := range rows(reference["handles"]) {
			id, _ := h["activityId"].(string)
			handles[id] = append(handles[id], h)
		}
		latest := map[string]Object{}
		for _, t := range rows(reference["activities"]) {
			id, _ := t["activityId"].(string)
			if latest[id] == nil || num(t["queuedAt"]) > num(latest[id]["queuedAt"]) || (num(t["queuedAt"]) == num(latest[id]["queuedAt"]) && num(t["startedAt"]) > num(latest[id]["startedAt"])) {
				latest[id] = t
			}
		}
		groups := map[string]Object{}
		completed := 0
		terminalHandles := true
		for _, h := range rows(reference["handles"]) {
			if h["status"] != "completed" {
				terminalHandles = false
			}
		}
		for id, t := range latest {
			if t["status"] != "completed" {
				continue
			}
			completed++
			runtimeID, _ := t["runtimeId"].(string)
			resource, _ := t["allocatedResourceId"].(string)
			class := referenceResourceClass(resource)
			key := runtimeID + "/" + class
			if groups[key] == nil {
				groups[key] = Object{"runtimeId": runtimeID, "resourceClass": class, "completedActivities": 0, "singleHandleActivities": 0, "scanTelemetrySamples": 0, "nativeAdmissionSamples": 0}
			}
			g := groups[key]
			g["completedActivities"] = int(num(g["completedActivities"])) + 1
			hs := handles[id]
			if len(hs) != 1 || hs[0]["status"] != "completed" {
				continue
			}
			g["singleHandleActivities"] = int(num(g["singleHandleActivities"])) + 1
			m := obj(hs[0]["metadata"])
			if num(m["remoteWorkspaceSnapshotSeconds"]) > 0 {
				g["scanTelemetrySamples"] = int(num(g["scanTelemetrySamples"])) + 1
			}
			if submitted := num(m["submittedAt"]); submitted > 0 && num(hs[0]["startedAt"]) >= submitted {
				g["nativeAdmissionSamples"] = int(num(g["nativeAdmissionSamples"])) + 1
			}
		}
		keys := []string{}
		for key := range groups {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		classes := []any{}
		for _, key := range keys {
			classes = append(classes, groups[key])
		}
		result = append(result, Object{
			"runId":                          run["id"],
			"status":                         run["status"],
			"distinctActivities":             len(latest),
			"completedDistinctActivities":    completed,
			"handleCount":                    len(rows(reference["handles"])),
			"eligibleCleanWholeRunReference": run["status"] == "completed" && completed == 58 && len(latest) == 58 && len(rows(reference["handles"])) == 58 && terminalHandles,
			"classes":                        classes,
		})
	}
	return Object{
		"issue":      "AKO-168",
		"references": result,
		"limitations": list(
			"Eligibility is not independence: a holdout cannot be the same run used to calibrate its case.",
			"Recovered references may supply single-attempt stage samples but not a clean whole-run makespan.",
			"Unknown resources remain explicitly unclassified rather than receiving borrowed coefficients.",
		),
	}
}

func referenceResourceClass(resource string) string {
	switch {
	case strings.Contains(resource, "fog-e2-micro"):
		return "e2-micro"
	case strings.Contains(resource, "e2-medium"):
		return "e2-medium"
	case strings.Contains(resource, "bora") || strings.Contains(resource, "diablo"):
		return "hpc"
	default:
		return "unclassified"
	}
}
