package montagetools

import "encoding/json"

// This is an exploratory wall-time sensitivity, never a queue-free fit or
// independent validation. It establishes how much a missing input scan changes
// the forecast without scaling the workflow makespan globally.
func observationSensitivity(request, real Object, evidenceDigest string) Object {
	check(obj(get(real, "run"))["status"] == "completed", "completed observation reference required")
	var seconds, bytes float64
	samples := 0
	for _, h := range rows(get(real, "handles")) {
		metadata := obj(h["metadata"])
		if h["status"] != "completed" || metadata["remoteWorkspaceSnapshotSeconds"] == nil {
			continue
		}
		var files []Object
		check(json.Unmarshal([]byte(str(get(metadata, "artifactObservationBefore"))), &files) == nil, "invalid observation manifest")
		var inputBytes float64
		for _, file := range files {
			inputBytes += num(get(file, "size"))
		}
		if inputBytes == 0 {
			continue
		}
		seconds += num(get(metadata, "remoteWorkspaceSnapshotSeconds"))
		bytes += inputBytes
		samples++
	}
	check(samples > 0 && bytes > 0, "no input observation samples")
	network := obj(get(request, "NetworkTopology"))
	metadata := obj(get(network, "metadata"))
	model := obj(get(metadata, "workspaceMaterializationModel"))
	model["inputObservationProfiles"] = []any{Object{"targetClass": "e2-micro", "source": "exploratory-observed-wall:" + evidenceDigest, "samples": samples, "secondsPerUniqueByte": seconds / bytes}}
	metadata["inputObservationSensitivity"] = Object{
		"referenceSHA256": evidenceDigest,
		"role":            "exploratory-wall-time-sensitivity-not-service-calibration",
		"samples":         samples,
		"limitations": list(
			"Includes unknown SSH wait and command overhead; not an accepted queue-free calibration.",
			"Recovered reference is not independent validation.",
			"Terminal observation and pruning are not represented by this input-only sensitivity.",
		),
	}
	return request
}
