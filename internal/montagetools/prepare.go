package montagetools

import (
	"os"
	"path/filepath"
	"strings"
)

func envDefault(key, fallback string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return v
}
func prepareRequests(args []string) {
	check(len(args) == 9, "prepare_montage_measured_requests BASELINE PROJECTION COMPUTE MATERIALIZATION GCP HPC SCOPE NETWORK OUTPUT_DIR")
	inputs := []Object{}
	for _, p := range args[:8] {
		m, _ := read(p)
		inputs = append(inputs, m)
	}
	baseline, p, compute, materialization, gcp, scope, network := inputs[0], inputs[1], inputs[2], inputs[3], inputs[4], inputs[6], inputs[7]
	_ = inputs[5]
	out := args[8]
	check(os.MkdirAll(out, 0700) == nil, "create output directory failed")
	save := func(name string, value any) { write(filepath.Join(out, name+".json"), value) }
	workflowName := envDefault("MONTAGE_WORKFLOW_NAME", "montage-reference-work-v1")
	revision := envDefault("MONTAGE_MODEL_REVISION", "v1")
	workflow := obj(get(baseline, "workflow"))
	sourceActivities := rows(get(workflow, "activities"))
	check(len(sourceActivities) == 58, "Expected complete Montage58")
	names := Object{}
	for _, a := range sourceActivities {
		names[str(a["id"])] = get(a, "name")
	}
	parents := Object{}
	for _, d := range rows(get(workflow, "dependencies")) {
		id := str(d["activityId"])
		parents[id] = append(arr(parents[id]), d)
	}
	activities := []any{}
	for _, a := range sourceActivities {
		id := str(a["id"])
		f := family(id)
		var work Object
		if p["activityComputeWork"] != nil {
			work = obj(get(obj(p["activityComputeWork"]), id))
			f = str(get(work, "activityFamily"))
		}
		check(get(obj(get(compute, "activityFamilyTypeIds")), f) == workflowName+"-"+strings.ReplaceAll(f, "_", "-"), "Unclassified family "+id)
		command := clone(obj(get(a, "command")))
		delete(command, "resolvedExecutable")
		metadata := clone(obj(a["metadata"]))
		metadata["workspaceProjection"] = get(obj(get(p, "activityModels")), id)
		metadata["calibrationSource"] = get(compute, "source")
		delete(metadata, "workspaceProjectionVolumes")
		if work != nil {
			metadata["computeWorkUnits"] = get(work, "computeWorkUnits")
			metadata["computeWorkUnit"] = get(work, "unit")
			metadata["computeWorkSource"] = get(work, "sourceImageSHA256")
		}
		depends := []any{}
		for _, d := range rows(parents[id]) {
			depends = append(depends, get(names, str(d["dependsOnActivityId"])))
		}
		seed, runtime, selector := metadata["workspaceSeedPath"], metadata["runtime"], metadata["resourceSelector"]
		if seed == nil {
			seed = ""
		}
		if runtime == nil {
			runtime = ""
		}
		if selector == nil {
			selector = ""
		}
		resources := obj(get(a, "resources"))
		activities = append(activities, Object{
			"name":              get(a, "name"),
			"activityType":      f,
			"metadata":          metadata,
			"command":           command,
			"simulation":        get(a, "simulation"),
			"cpuLimit":          numericString(get(resources, "cpu")),
			"memoryLimit":       numericString(get(resources, "memoryBytes")),
			"dependsOn":         depends,
			"workspaceSeedPath": seed,
			"expectedOutputs":   metadata["expectedOutputs"],
			"runtime":           runtime,
			"resourceSelector":  selector,
		})
	}
	save("workflow-import", Object{"name": workflowName, "spec": Object{"namespace": "montage-measured-20261011", "activities": activities}})
	relay := Object{
		"id":                   get(materialization, "relayResourceId"),
		"environmentVersionId": get(obj(get(gcp, "version")), "id"),
		"name":                 "Existing Akoflow relay controller (model-only, not schedulable)",
		"type":                 "local_machine",
		"cpuCores":             8,
		"cpuCapacity":          8,
		"memoryBytes":          int64(8) * 1024 * 1024 * 1024,
		"computeSpeedup":       1,
		"schedulable":          false,
		"metadata":             Object{"modelOnly": true, "role": "transfer-relay", "provision": false},
	}
	save("relay-resource", relay)
	network["id"] = "montage-reference-network-20261011-" + revision
	network["name"] = "Measured workspace materialization with explicit controller relay"
	meta := obj(network["metadata"])
	meta["workspaceMaterializationModel"] = materialization
	meta["calibrationSource"] = get(materialization, "source")
	meta["linkBandwidthBasis"] = "Inherited topology assumptions; phase flow caps use measured historical effective rates"
	network["metadata"] = meta
	links := arr(get(network, "links"))
	for _, l := range rows(links) {
		m := obj(l["metadata"])
		m["sourceTopologyLinkId"] = get(l, "id")
		l["metadata"] = m
		l["id"] = str(network["id"]) + "-" + str(l["id"])
		l["topologyId"] = network["id"]
	}
	for _, id := range sortedKeys(obj(get(materialization, "resources"))) {
		if id == relay["id"] {
			continue
		}
		links = append(links, Object{
			"id":                     str(network["id"]) + "-relay-" + id,
			"topologyId":             network["id"],
			"sourceResourceId":       relay["id"],
			"targetResourceId":       id,
			"bandwidthBitsPerSecond": 200000000,
			"latencySeconds":         0.05,
			"pricePerByte":           0,
			"bidirectional":          true,
			"sharingPolicy":          "SHARED",
			"metadata":               Object{"measurementStatus": "assumed-inherited-200Mbps", "networkCostCalibrated": false},
		})
	}
	network["links"] = links
	scope["id"] = "montage-reference-scope-20261011-" + revision
	scope["networkTopologyId"] = network["id"]
	network["executionScopeId"] = scope["id"]
	save("scope", scope)
	save("network", network)
	units := "reference-work-v1"
	if p["activityComputeWork"] != nil {
		units = "compute-work-v1"
	}
	save("planning-session", Object{
		"id":                envDefault("MONTAGE_PLANNING_ID", "montage-reference-planning-20261011-v2"),
		"workflowVersionId": workflowName + "-v1",
		"executionScopeId":  scope["id"],
		"networkTopologyId": network["id"],
		"algorithms":        []any{Object{"id": "prism-time", "configuration": Object{"requireMeasuredProfiles": true}}},
		"configuration": Object{
			"calibrationSource":                         compute["source"],
			"observeWorkspaceStorageBudgets":            true,
			"activityProfileSnapshot":                   compute["activityProfiles"],
			"requireMeasuredProfiles":                   true,
			"computeCalibrationUnits":                   units,
			"computeCalibrationIncludesPhysicalSharing": true,
			"additionalPairwiseSlowdown":                "not-applied-to-contention-conditioned-wall-profiles",
		},
	})
}
