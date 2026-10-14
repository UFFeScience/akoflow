package montagetools

import (
	"math"
	"sort"
	"strings"
)

func fixedRequest(original, model, observed Object, id string) Object {
	r := clone(original)
	check(obj(get(r, "run"))["mode"] == "simulation", "Simulation-only request required")
	assigned, actual := Object{}, Object{}
	for _, a := range rows(obj(r["plan"])["assignments"]) {
		assigned[str(a["activityId"])] = a["resourceId"]
	}
	for _, a := range rows(observed["activities"]) {
		actual[str(a["activity_id"])] = a["resource_id"]
	}
	check(eq(assigned, actual), "Fixed observed placement differs from plan")
	obj(r["run"])["id"] = id
	obj(get(obj(get(r, "networkTopology")), "metadata"))["workspaceMaterializationModel"] = model
	return r
}
func errorPercent(predicted, observed float64) any {
	if observed <= 0 {
		return nil
	}
	return 100 * (predicted - observed) / observed
}
func compare(observed, simulated Object, role string, model Object) Object {
	check(role == "original-frozen-forecast" || role == "historical-calibration-completed-run-holdout" || role == "calibration-fit-not-independent-validation", "Unknown comparison role")
	predicted := Object{}
	for _, r := range rows(get(simulated, "transfers")) {
		predicted[str(r["producerActivityId"])+"\x00"+str(r["consumerActivityId"])] = r
	}
	resources := Object{}
	for _, a := range rows(observed["activities"]) {
		resources[str(a["activity_id"])] = a["resource_id"]
	}
	resultRows, mismatch := compareDependencyRoutes(observed, predicted, resources)
	makespan, real := num(get(obj(get(simulated, "executed")), "makespanSeconds")), num(get(obj(get(observed, "real_api_reported")), "makespanSeconds"))
	percent := errorPercent(makespan, real)
	dominant := append([]any{}, resultRows...)
	sort.SliceStable(dominant, func(i, j int) bool {
		return num(obj(dominant[i])["observedDependencyPhaseSeconds"]) > num(obj(dominant[j])["observedDependencyPhaseSeconds"])
	})
	if len(dominant) > 15 {
		dominant = dominant[:15]
	}
	predBytes, realBytes := sum(rows(resultRows), "predictedNetworkBytes"), sum(rows(resultRows), "observedNetworkBytes")
	limitation := "Observed comparison run was not used to fit these transfer coefficients; this remains a retrospective test, not a new prospective real validation."
	if role == "calibration-fit-not-independent-validation" {
		limitation = "This comparison reuses calibration data and is not independent validation."
	}
	return Object{
		"schemaVersion":                  1,
		"comparisonRole":                 role,
		"observedRunId":                  get(observed, "run_id"),
		"modelSource":                    get(model, "source"),
		"simulatedMakespanSeconds":       makespan,
		"observedMakespanSeconds":        real,
		"makespanErrorPercent":           percent,
		"withinInitialMakespanTolerance": percent != nil && math.Abs(num(percent)) <= 20,
		"predictedNetworkBytes":          predBytes,
		"observedNetworkBytes":           realBytes,
		"networkBytesErrorPercent":       errorPercent(predBytes, realBytes),
		"routeMismatchCount":             mismatch,
		"dominantTransfers":              dominant,
		"dependencies":                   resultRows,
		"accepted":                       false,
		"limitations": list(
			"Makespan tolerance alone is insufficient; dominant transfer residuals must also be reviewed.",
			"Historical receiver phases may include unseparated queues; destination bulk pools must remain disabled until service is separately identified.",
			"No summed parallel phase durations are treated as makespan.",
			"Compute, ordering and provisioning assumptions retained from the frozen original request.",
			limitation,
		),
	}
}
func sourceCapacity(diagnostic Object, sourceID, targetClass string) Object {
	intervals := []Object{}
	boundaries := map[float64]bool{}
	for _, t := range rows(get(diagnostic, "artifactTransferRuns")) {
		r := obj(t["route"])
		if t["status"] != "completed" || r["strategy"] != "gateway" || r["sourceResourceId"] != sourceID || num(r["sourcePackExecutionSeconds"]) <= 0 || num(r["archiveBytes"]) <= 0 {
			continue
		}
		if resourceClass(str(get(r, "targetResourceId"))) != targetClass {
			continue
		}
		for _, p := range rows(get(r, "phases")) {
			if p["name"] == "pack-and-receive" && p["finishedAt"] != nil {
				finish, seconds := num(p["finishedAt"]), num(r["sourcePackExecutionSeconds"])
				intervals = append(intervals, Object{"start": finish - seconds, "finish": finish, "rate": num(r["archiveBytes"]) / seconds})
				boundaries[finish-seconds] = true
				boundaries[finish] = true
				break
			}
		}
	}
	check(len(intervals) >= 3, "Insufficient completed source-capacity observations")
	points := []float64{}
	for p := range boundaries {
		points = append(points, p)
	}
	sort.Float64s(points)
	windows := []Object{}
	total := 0.0
	maximum := 0
	for i := 1; i < len(points); i++ {
		a, b := points[i-1], points[i]
		rate, n := 0.0, 0
		for _, v := range intervals {
			if num(v["start"]) <= a && num(v["finish"]) >= b {
				rate += num(v["rate"])
				n++
			}
		}
		if n > 0 {
			windows = append(windows, Object{"seconds": b - a, "bytesPerSecond": rate, "concurrentCommands": n})
			total += b - a
			if n > maximum {
				maximum = n
			}
		}
	}
	check(total > 0, "Invalid source capacity interval")
	sort.SliceStable(windows, func(i, j int) bool { return num(windows[i]["bytesPerSecond"]) < num(windows[j]["bytesPerSecond"]) })
	cumulative, median := 0.0, 0.0
	for _, w := range windows {
		cumulative += num(w["seconds"])
		if cumulative >= total/2 {
			median = num(w["bytesPerSecond"])
			break
		}
	}
	return Object{
		"samples":                              len(intervals),
		"busySeconds":                          total,
		"concurrentCommandsMaximum":            maximum,
		"durationWeightedMedianBytesPerSecond": median,
		"minimumBytesPerSecond":                windows[0]["bytesPerSecond"],
		"maximumBytesPerSecond":                windows[len(windows)-1]["bytesPerSecond"],
		"interpretation":                       "Aggregate source pack/ingress service, NOT NIC bandwidth; assumes uniform byte progress within each queue-free command.",
		"limitations":                          list("Small auxiliary diagnostic sample; capacity is conditioned on workload and concurrent source work.", "Admission queue excluded from command intervals."),
	}
}
func applyCapacity(request Object, source string, capacity Object) Object {
	r := clone(request)
	topology := obj(get(r, "networkTopology"))
	model := obj(get(obj(get(topology, "metadata")), "workspaceMaterializationModel"))
	relay := get(model, "relayResourceId")
	links := arr(get(topology, "links"))
	var physical Object
	for _, v := range links {
		l := obj(v)
		if l["sourceResourceId"] == relay && l["targetResourceId"] == source && l["bidirectional"] == true {
			physical = l
			break
		}
	}
	check(physical != nil, "Expected explicit bidirectional relay endpoint link")
	physical["bidirectional"] = false
	measured := clone(physical)
	measured["id"] = str(physical["id"]) + "-measured-source-service"
	measured["sourceResourceId"] = source
	measured["targetResourceId"] = relay
	measured["bandwidthBitsPerSecond"] = 8 * num(capacity["durationWeightedMedianBytesPerSecond"])
	measured["sharingPolicy"] = "SHARED"
	measured["metadata"] = Object{"measurementBasis": "combined-source-pack-and-ingress-service-not-nic", "evidence": capacity}
	topology["links"] = append(links, measured)
	return r
}
func supplementSource(c, diagnostic Object, d string) Object {
	manifests, snapshots := Object{}, Object{}
	for _, h := range rows(get(diagnostic, "handles")) {
		if h["status"] == "completed" && h["exitCode"] != nil && num(h["exitCode"]) == 0 {
			id := str(get(h, "activityId"))
			m := files(get(obj(get(h, "artifacts")), "finalSnapshot"))
			manifests[id] = m
			snapshots[id] = fileBytes(m)
		}
	}
	samples := []Object{}
	completed := []Object{}
	for _, t := range rows(get(diagnostic, "artifactTransferRuns")) {
		if t["status"] != "completed" {
			continue
		}
		r := obj(t["route"])
		if r["producerActivityId"] != nil {
			completed = append(completed, r)
		}
		if r["strategy"] != "gateway" || r["sourcePackExecutionSeconds"] == nil || r["sourcePackPreflightServiceSeconds"] == nil || r["archiveBytes"] == nil || snapshots[str(r["producerActivityId"])] == nil {
			continue
		}
		if resourceClass(str(r["sourceResourceId"])) != "hpc" || resourceClass(str(r["targetResourceId"])) != "e2-micro" {
			continue
		}
		samples = append(samples, Object{
			"seconds":        r["sourcePackExecutionSeconds"],
			"archiveBytes":   r["archiveBytes"],
			"workspaceBytes": snapshots[str(r["producerActivityId"])],
			"wait":           num(r["sourcePackSshChannelWaitSeconds"]),
			"preflight":      r["sourcePackPreflightServiceSeconds"],
		})
	}
	check(len(samples) > 0, "No successful separately measured source service")
	updated := clone(c)
	var phase Object
	for _, p := range rows(get(updated, "materializationPhaseObservations")) {
		if p["strategy"] == "gateway" && p["sourceClass"] == "hpc" && p["targetClass"] == "e2-micro" && p["phase"] == "pack-and-receive" {
			phase = p
			break
		}
	}
	check(phase != nil, "Completed primary reference lacks this route class")
	phase["seconds"] = summary(vals(samples, "seconds"))
	phase["secondsBases"] = list("source-command-execution-excludes-channel-queue")
	phase["pooledArchiveServiceRate"] = pooledRate(samples, "archiveBytes")
	phase["pooledWorkspaceServiceRate"] = pooledRate(samples, "workspaceBytes")
	phase["sourceChannelWaitSeconds"] = summary(vals(samples, "wait"))
	phase["sourcePreflightServiceSeconds"] = summary(vals(samples, "preflight"))
	evidence := Object{
		"runId":                    get(obj(get(diagnostic, "run")), "id"),
		"sha256":                   d,
		"completedTransferSamples": len(samples),
		"role":                     "completed commands from interrupted diagnostic; not independent validation",
		"queueNotAddedToService":   true,
	}
	updated["auxiliarySourceServiceEvidence"] = evidence
	merges := auxiliaryHPCMergeSamples(completed, manifests, snapshots)
	check(len(merges) > 0, "Queue-free auxiliary HPC merge service missing")
	var merge Object
	for _, m := range rows(get(updated, "mergeRateObservations")) {
		if m["targetClass"] == "hpc" {
			merge = m
			break
		}
	}
	check(merge != nil, "Primary HPC merge reference missing")
	rates := Object{}
	for _, key := range []string{"checkSecondsPerInputByte", "copySecondsPerUniqueByte", "verifySecondsPerUniqueByte"} {
		rates[key] = summary(vals(merges, key))
	}
	merge["observations"] = rates
	merge["commandOverheadSeconds"] = summary(vals(merges, "overhead"))
	merge["channelWaitSeconds"] = summary(vals(merges, "wait"))
	evidence["completedHPCMergeSamples"] = len(merges)
	return updated
}

func compareDependencyRoutes(observed, predicted, resources Object) ([]any, int) {
	resultRows := []any{}
	mismatch := 0
	for _, r := range rows(get(observed, "routes")) {
		if r["producer_activity_id"] == nil {
			continue
		}
		producer, consumer := str(r["producer_activity_id"]), str(r["consumer_activity_id"])
		estimate := obj(predicted[producer+"\x00"+consumer])
		service, local := 0.0, resources[producer] == resources[consumer] && num(r["network_bytes"]) == 0
		for _, p := range rows(get(r, "phases")) {
			if !strings.HasPrefix(str(p["name"]), "shared-") {
				service += num(get(p, "durationSeconds"))
				local = false
			}
		}
		if r["strategy"] == "runtime-local" {
			service = 0
		}
		canonical := r["strategy"]
		if local {
			canonical = "runtime-local"
		}
		strategy := "runtime-local"
		duration, network, wait := 0.0, 0.0, 0.0
		var percent any
		if len(estimate) > 0 {
			strategy = str(get(estimate, "strategy"))
			duration = num(get(estimate, "durationSeconds"))
			network = num(estimate["networkBytes"])
			percent = errorPercent(duration, service)
			for _, p := range rows(get(obj(get(estimate, "route")), "phases")) {
				wait += num(p["serviceWaitSeconds"])
			}
		}
		if canonical != strategy {
			mismatch++
		}
		resultRows = append(resultRows, Object{
			"producer":                       producer,
			"consumer":                       consumer,
			"observedStrategy":               canonical,
			"executorStrategy":               r["strategy"],
			"predictedStrategy":              strategy,
			"observedNetworkBytes":           num(r["network_bytes"]),
			"predictedNetworkBytes":          network,
			"observedDependencyPhaseSeconds": service,
			"predictedDependencySeconds":     duration,
			"durationErrorPercent":           percent,
			"predictedServiceWaitSeconds":    wait,
		})
	}
	return resultRows, mismatch
}

func auxiliaryHPCMergeSamples(completed []Object, manifests, snapshots Object) []Object {
	merges := []Object{}
	for _, g := range groups(completed, "consumerActivityId") {
		consumer := g.key[0]
		if manifests[consumer] == nil {
			continue
		}
		r := g.rows[0]
		if resourceClass(str(get(r, "targetResourceId"))) != "hpc" || r["mergeCommandOverheadSeconds"] == nil || r["mergeSshChannelWaitSeconds"] == nil {
			continue
		}
		parents := Object{}
		valid := true
		for _, p := range g.rows {
			id := str(p["producerActivityId"])
			parents[id] = true
			valid = valid && manifests[id] != nil
		}
		if !valid {
			continue
		}
		sources := []Object{}
		total := 0.0
		for _, p := range sortedKeys(parents) {
			sources = append(sources, obj(manifests[p]))
			total += num(snapshots[p])
		}
		unique := fileBytes(unionFiles(sources))
		if total <= 0 || unique <= 0 {
			continue
		}
		merges = append(merges, Object{
			"checkSecondsPerInputByte":   num(get(r, "mergeCheckSeconds")) / total,
			"copySecondsPerUniqueByte":   num(get(r, "mergeCopySeconds")) / unique,
			"verifySecondsPerUniqueByte": num(get(r, "mergeVerifySeconds")) / unique,
			"overhead":                   r["mergeCommandOverheadSeconds"],
			"wait":                       r["mergeSshChannelWaitSeconds"],
		})
	}
	return merges
}
