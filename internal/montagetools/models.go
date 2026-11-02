package montagetools

import (
	"math"
	"strings"
)

func median(row Object, key string, positive bool) float64 {
	s := obj(get(row, key))
	check(num(s["samples"]) > 0, "Missing measured "+key)
	v := num(get(s, "median"))
	check(!math.IsInf(v, 0) && !math.IsNaN(v) && v >= 0 && (!positive || v > 0), "Invalid measured "+key)
	return v
}
func serviceRate(row Object, pooled, legacy string) float64 {
	if row[pooled] == nil {
		return median(row, legacy, true)
	}
	r := obj(row[pooled])
	v := num(get(r, "bytesPerSecond"))
	check(num(r["samples"]) > 0 && v > 0 && !math.IsInf(v, 0) && !math.IsNaN(v), "Invalid pooled measured service rate")
	return v
}
func freezeMaterialization(c Object, d string, resources Object, relay string) Object {
	check(get(c, "workspaceVolumeBasis") == "producer-final-snapshot-manifest", "Manifest-based workspace volumes required")
	check(get(obj(get(resources, relay)), "domain") == "relay", "Explicit relay resource required")
	source := str(get(c, "baselineRunId")) + ":sha256:" + d
	profiles := []any{}
	for _, g := range groups(rows(get(c, "materializationPhaseObservations")), "strategy", "sourceClass", "targetClass") {
		strategy, from, to := g.key[0], g.key[1], g.key[2]
		check(strategy == "gateway" || strategy == "direct-runtime", "Unsupported measured strategy")
		phases := Object{}
		for _, p := range g.rows {
			phases[str(p["phase"])] = p
		}
		uploadName, receiverName := "peer-private-upload", "peer-receiver-verify-and-extract"
		if strategy == "gateway" {
			uploadName, receiverName = "archive-upload", "receiver-verify-and-extract"
		}
		upload := obj(get(phases, uploadName))
		targetPool, sourcePool := false, false
		for _, id := range sortedKeys(resources) {
			r := obj(resources[id])
			if r["bulkServiceGroupId"] != nil && r["bulkServiceGroupId"] != "" {
				targetPool = targetPool || r["class"] == to
				sourcePool = sourcePool || r["class"] == from
			}
		}
		if targetPool {
			check(eq(obj(get(phases, receiverName))["secondsBases"], list("phase-command-execution-excludes-channel-queue")), "Receiver "+from+"->"+to+" requires queue-free command service before adding a bulk pool")
		}
		p := Object{
			"strategy":                    strategy,
			"sourceClass":                 from,
			"targetClass":                 to,
			"source":                      source,
			"samples":                     get(obj(get(upload, "seconds")), "samples"),
			"archiveToWorkspaceRatio":     median(upload, "archiveToWorkspaceRatio", true),
			"uploadArchiveBytesPerSecond": serviceRate(upload, "pooledArchiveServiceRate", "effectiveArchiveBytesPerSecond"),
		}
		if strategy == "gateway" {
			pack := obj(get(phases, "pack-and-receive"))
			if sourcePool {
				check(eq(pack["secondsBases"], list("source-command-execution-excludes-channel-queue")), "Gateway "+from+"->"+to+" requires queue-free source service measurements before adding a bulk pool")
				check(pack["sourcePreflightServiceSeconds"] != nil, "Gateway "+from+"->"+to+" requires separately measured preflight service")
			}
			p["ingressArchiveBytesPerSecond"] = serviceRate(pack, "pooledArchiveServiceRate", "effectiveArchiveBytesPerSecond")
			if pack["sourcePreflightServiceSeconds"] != nil {
				p["probeSeconds"] = median(pack, "sourcePreflightServiceSeconds", false)
			}
			p["ingressIncludesSourcePreparation"] = true
			p["validationSecondsPerByte"] = 1 / serviceRate(obj(get(phases, "archive-validation")), "pooledWorkspaceServiceRate", "effectiveWorkspaceBytesPerSecond")
		} else {
			p["probeSeconds"] = median(obj(get(phases, "peer-authenticated-connectivity-probe")), "seconds", false)
			p["snapshotSecondsPerByte"] = 1 / serviceRate(obj(get(phases, "peer-snapshot-acquire")), "pooledWorkspaceServiceRate", "effectiveWorkspaceBytesPerSecond")
		}
		p["receiverSecondsPerByte"] = 1 / serviceRate(obj(get(phases, receiverName)), "pooledWorkspaceServiceRate", "effectiveWorkspaceBytesPerSecond")
		profiles = append(profiles, p)
	}
	merges := []any{}
	for _, r := range rows(get(c, "mergeRateObservations")) {
		rates := obj(get(r, "observations"))
		p := Object{"targetClass": get(r, "targetClass"), "source": source, "samples": get(obj(get(rates, "checkSecondsPerInputByte")), "samples")}
		for _, k := range sortedKeys(rates) {
			p[k] = median(rates, k, false)
		}
		if r["commandOverheadSeconds"] != nil {
			p["commandOverheadSeconds"] = median(r, "commandOverheadSeconds", false)
		}
		merges = append(merges, p)
	}
	return Object{"schemaVersion": 1, "source": source, "relayResourceId": relay, "resources": resources, "profiles": profiles, "mergeProfiles": merges}
}
func freezeCompute(c Object, d string, resources Object, namespace string) Object {
	independent := c["computeWorkBasis"] == "immutable-dataset-independent-work-v1"
	source := str(get(c, "baselineRunId")) + ":sha256:" + d
	families := Object{}
	for _, r := range rows(get(c, "computeObservations")) {
		families[str(get(r, "activityFamily"))] = true
	}
	typeIDs := Object{}
	for _, f := range sortedKeys(families) {
		typeIDs[f] = namespace + "-" + strings.ReplaceAll(f, "_", "-")
	}
	profiles := []any{}
	for _, r := range rows(c["computeObservations"]) {
		coefficient := median(r, "wallSecondsPerReferenceSecond", true)
		stats := obj(r["wallSecondsPerReferenceSecond"])
		for _, id := range sortedKeys(resources) {
			resource := obj(resources[id])
			if resource["class"] != r["resourceClass"] {
				continue
			}
			if r["allowedResourceIds"] != nil {
				allowed := false
				for _, v := range arr(r["allowedResourceIds"]) {
					allowed = allowed || v == id
				}
				if !allowed {
					continue
				}
			}
			modelVersion, units := "reference-work-v1", "wall-seconds-per-reference-second"
			if independent {
				modelVersion, units = "compute-work-v1", "wall-seconds-per-independent-work-unit"
			}
			s := r["source"]
			if s == nil {
				s = source
			}
			f := str(r["activityFamily"])
			profiles = append(profiles, Object{
				"id":             namespace + "-" + f + "-" + id,
				"activityTypeId": typeIDs[f],
				"resourceId":     id,
				"runtimeSeconds": coefficient,
				"modelVersion":   modelVersion,
				"source":         s,
				"sampleSize":     get(stats, "samples"),
				"metadata":       Object{"units": units, "observedRange": []any{stats["minimum"], stats["maximum"]}, "includesPhysicalSharing": true, "taskRuntimeFallbackSamples": get(r, "taskRuntimeFallbackSamples")},
			})
		}
	}
	return Object{
		"schemaVersion":         1,
		"source":                source,
		"activityFamilyTypeIds": typeIDs,
		"activityProfiles":      profiles,
		"limitations": list(
			"Class-level historical wall rates; no per-activity replay.",
			"Unobserved family/class pairs have no measured profile.",
			"Includes physical sharing: additional interference factors require separate evidence.",
		),
	}
}
