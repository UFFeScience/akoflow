package montagetools

import (
	"math"
	"sort"
	"time"
)

func compareActivityTimes(request, real, simulation Object) Object {
	plan := map[string]Object{}
	for _, a := range rows(obj(request["Plan"])["assignments"]) {
		plan[str(a["activityId"])] = a
	}
	sim := map[string]Object{}
	for _, a := range rows(simulation["tasks"]) {
		sim[str(a["activityId"])] = a
	}
	logicalReady := map[string]float64{}
	for _, dependency := range rows(obj(request["Workflow"])["dependencies"]) {
		producer := sim[str(dependency["dependsOnActivityId"])]
		check(producer != nil, "missing simulated producer publication")
		consumer := str(dependency["activityId"])
		logicalReady[consumer] = math.Max(logicalReady[consumer], num(producer["finishedAt"]))
	}
	completed := map[string]float64{}
	for _, event := range rows(real["events"]) {
		if event["eventType"] != "activity.completed" {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, str(event["occurredAt"]))
		check(err == nil, "invalid completion timestamp")
		completed[str(event["aggregateId"])] = float64(at.UnixNano()) / 1e9
	}
	handles := map[string]int{}
	for _, h := range rows(real["handles"]) {
		handles[str(h["activityId"])]++
	}
	results := []any{}
	var computeErrors, wallErrors []any
	for _, activity := range rows(real["activities"]) {
		id := str(activity["activityId"])
		a, s := plan[id], sim[id]
		check(a != nil && s != nil && activity["status"] == "completed" && s["status"] == "completed", "missing completed matched activity")
		samePlacement := activity["allocatedResourceId"] == s["allocatedResourceId"]
		metadata := obj(activity["metadata"])
		finish, hasCompletion := completed[str(activity["id"])]
		ready := num(activity["readyAt"])
		comparable := handles[id] == 1 && hasCompletion && ready > 0 && finish >= ready
		realCompute, simCompute := num(activity["runtimeSeconds"]), num(s["runtimeSeconds"])
		computeError := errorPercent(simCompute, realCompute)
		simMetadata := obj(s["metadata"])
		simReady := logicalReady[id]
		record := Object{"activityId": id, "resourceClass": resourceClass(str(activity["allocatedResourceId"])),
			"samePlacement": samePlacement, "observedResourceId": activity["allocatedResourceId"], "simulatedResourceId": s["allocatedResourceId"],
			"plannedComputeSeconds": a["predictedRuntimeSeconds"], "simulatedComputeSeconds": simCompute, "observedComputeSeconds": realCompute,
			"computeErrorPercent": computeError, "singleAttemptComparable": comparable,
			"observedPreparationElapsedSeconds": metadata["transferElapsedSeconds"], "observedTransferWorkSecondsNotElapsed": activity["transferSeconds"],
			"observedLaunchWaitSeconds": metadata["launchWaitSeconds"], "observedQueueSecondsOverlapsPreparation": activity["queueSeconds"],
			"simulatedLogicalReadyAt": simReady, "simulatedReadyAtAfterTransfers": s["readyAt"],
			"simulatedPreparationElapsedSeconds":         num(simMetadata["workspacePreparationFinishedAt"]) - simReady,
			"simulatedPostPreparationWaitSeconds":        num(s["startedAt"]) - num(simMetadata["workspacePreparationFinishedAt"]),
			"simulatedPublicationSeconds":                simMetadata["workspacePublicationSeconds"],
			"simulatedQueueSecondsNotDirectlyComparable": s["queueSeconds"],
			"observedBeforePreparationWaitSeconds":       num(metadata["preparationStartedAt"]) - ready,
			"observedPostPreparationWaitSeconds":         num(activity["startedAt"]) - num(metadata["preparationFinishedAt"]),
			"observedRuntimeWallSeconds":                 num(activity["finishedAt"]) - num(activity["startedAt"]),
			"simulatedReadyToPublishedSeconds":           num(s["finishedAt"]) - simReady}
		if comparable {
			realWall := finish - ready
			simWall := num(s["finishedAt"]) - simReady
			record["observedPublicationAndPollSeconds"] = finish - num(activity["finishedAt"])
			record["observedReadyToPublishedSeconds"] = realWall
			record["readyToPublishedErrorPercent"] = errorPercent(simWall, realWall)
			computeErrors = append(computeErrors, math.Abs(num(computeError)))
			if samePlacement {
				wallErrors = append(wallErrors, math.Abs(num(errorPercent(simWall, realWall))))
			}
		} else {
			record["excludedWallReason"] = "recovered attempt or missing matched publication evidence"
		}
		results = append(results, record)
	}
	sort.Slice(results, func(i, j int) bool { return str(obj(results[i])["activityId"]) < str(obj(results[j])["activityId"]) })
	within := func(values []any) int {
		n := 0
		for _, v := range values {
			if num(v) <= 20 {
				n++
			}
		}
		return n
	}
	return Object{
		"referenceRunId":                       obj(real["run"])["id"],
		"role":                                 "per-activity-historical-comparison-not-independent-validation",
		"activityCount":                        len(results),
		"singleAttemptComparableCount":         len(computeErrors),
		"samePlacementWallComparableCount":     len(wallErrors),
		"computeWithin20PercentCount":          within(computeErrors),
		"readyToPublishedWithin20PercentCount": within(wallErrors),
		"absoluteComputeErrorPercent":          summary(computeErrors),
		"absoluteReadyToPublishedErrorPercent": summary(wallErrors),
		"activities":                           results,
		"limitations": list(
			"Queue overlaps preparation; these counters must not be added.",
			"Transfer work sums parallel dependencies and is not consumer elapsed time.",
			"Ready-to-published includes preparation, lane waiting, launch, compute and publication, but not time before readiness.",
			"Recovered activities are excluded from single-attempt wall comparisons.",
			"Publication and input-observation coefficients were fitted on this reference; their agreement is in-sample, not independent validation.",
		),
	}
}
