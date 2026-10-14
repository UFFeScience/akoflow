package montagetools

import (
	"os"
	"path/filepath"
)

// Prepare only: no API submission. A frozen exploratory forecast is not a
// passed validation gate. The independent run collects missing service data.
func prepareInstrumentedRound(request, simulation Object, runID, planID string) Object {
	check(runPattern.MatchString(runID) && runPattern.MatchString(planID), "invalid fresh round identity")
	request = clone(request)
	check(len(arr(request["RuntimeAllocations"])) == 0, "fresh round must not reuse concrete allocations")
	activities := rows(obj(get(request, "Workflow"))["activities"])
	assignments := rows(obj(get(request, "Plan"))["assignments"])
	tasks := rows(get(simulation, "tasks"))
	check(len(activities) == 58 && len(assignments) == 58 && len(tasks) == 58, "complete Montage58 forecast required")
	planned := map[string]string{}
	for _, assignment := range assignments {
		id := str(get(assignment, "activityId"))
		check(planned[id] == "", "duplicate assignment")
		planned[id] = str(get(assignment, "resourceId"))
	}
	seen := map[string]bool{}
	for _, task := range tasks {
		id := str(get(task, "activityId"))
		check(!seen[id] && planned[id] != "" && planned[id] == task["allocatedResourceId"] && task["status"] == "completed", "simulation does not match complete planned placement")
		seen[id] = true
	}
	for _, activity := range activities {
		check(seen[str(get(activity, "id"))], "unplanned workflow activity")
	}
	plan := obj(request["Plan"])
	plan["id"] = planID
	for _, assignment := range assignments {
		assignment["planId"] = planID
		assignment["id"] = planID + "-" + str(assignment["activityId"])
	}
	for _, action := range rows(plan["lifecycleActions"]) {
		action["planId"] = planID
	}
	metadata := obj(plan["metadata"])
	metadata["experimentRole"] = "fresh-instrumented-round-not-yet-validated"
	metadata["dominantTransferGateAccepted"] = false
	plan["metadata"] = metadata
	request["Run"] = Object{"id": runID, "schedulePlanId": planID, "mode": "real", "seed": 1}
	return request
}

func writeInstrumentedRound(request, simulation Object, rawRequest, rawSimulation []byte, runID, planID, out string) {
	_, err := os.Stat(out)
	check(os.IsNotExist(err), "round output already exists or inaccessible")
	prepared := prepareInstrumentedRound(request, simulation, runID, planID)
	check(os.Mkdir(out, 0700) == nil, "create round directory failed")
	write(filepath.Join(out, "real-request.json"), prepared)
	write(filepath.Join(out, "simulation-reference.json"), simulation)
	write(filepath.Join(out, "frozen-provenance.json"), Object{
		"issue": "AKO-158", "runId": runID, "planId": planID,
		"inputRequestSHA256": digest(rawRequest), "simulationSHA256": digest(rawSimulation),
		"realRequestSHA256":            digest(encode(prepared)),
		"planMakespanSeconds":          obj(obj(prepared["Plan"])["predicted"])["makespanSeconds"],
		"simgridMakespanSeconds":       obj(simulation["executed"])["makespanSeconds"],
		"role":                         "frozen-exploratory-forecast-for-new-measurements",
		"dominantTransferGateAccepted": false, "realSubmitted": false,
	})
}
