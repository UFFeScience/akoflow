package montagetools

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func scientific(t *testing.T, name string) Object {
	t.Helper()
	m, _ := read(filepath.Join("..", "..", "experiments", "montage-micro-hpc-e2", name))
	return m
}
func bad(t *testing.T, fn func() Object) { t.Helper(); _, err := attempt(fn); require.Error(t, err) }
func TestGoldenMigrationParity(t *testing.T) {
	a := scientific(t, "docker-image-real-v2-results.json")
	p := scientific(t, "workspace-projection-v2.json")
	c := calibration(a, str(p["auditedEvidenceSHA256"]), p)
	want, _ := read("testdata/calibration.json")
	equivalent(t, normal(want), normal(c))
	resources := scientific(t, "materialization-resource-model.json")
	m := freezeMaterialization(c, "migration-fixture-digest", resources, "montage-relay-controller")
	want, _ = read("testdata/materialization.json")
	equivalent(t, normal(want), normal(m))
	compute := freezeCompute(c, "migration-fixture-digest", resources, "migration-fixture")
	want, _ = read("testdata/compute.json")
	sortScientificRows(compute, "activityProfiles", "id")
	sortScientificRows(want, "activityProfiles", "id")
	equivalent(t, normal(want), normal(compute))
}
func equivalent(t *testing.T, want, got any) {
	t.Helper()
	switch w := want.(type) {
	case float64:
		g, ok := got.(float64)
		require.True(t, ok)
		require.InDelta(t, w, g, math.Max(1, math.Abs(w))*1e-12)
	case map[string]any:
		g, ok := got.(map[string]any)
		require.True(t, ok)
		require.Len(t, g, len(w))
		for k, v := range w {
			require.Contains(t, g, k)
			equivalent(t, v, g[k])
		}
	case []any:
		g, ok := got.([]any)
		require.True(t, ok)
		require.Len(t, g, len(w))
		for i, v := range w {
			equivalent(t, v, g[i])
		}
	default:
		require.Equal(t, want, got)
	}
}
func sortScientificRows(m Object, key, id string) {
	rs := rows(m[key])
	sortRows(rs, id)
	out := []any{}
	for _, r := range rs {
		out = append(out, r)
	}
	m[key] = out
}
func TestRootComputeSupplement(t *testing.T) {
	c := Object{"computeWorkBasis": "immutable-dataset-independent-work-v1", "computeObservations": []any{}}
	p := Object{"activityComputeWork": Object{}}
	request := Object{"workflow": Object{"activities": []any{}}, "plan": Object{"assignments": []any{}}}
	response := Object{"run": Object{"id": "training", "status": "failed"}, "activityAttempts": []any{}, "handles": []any{}}
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("mprojectid%07d", i)
		obj(p["activityComputeWork"])[id] = Object{"activityName": id, "activityFamily": "mproject_red", "computeWorkUnits": 6.0}
		w := obj(request["workflow"])
		w["activities"] = append(arr(w["activities"]), Object{"id": id, "name": id})
		plan := obj(request["plan"])
		plan["assignments"] = append(arr(plan["assignments"]), Object{"activityId": id, "resourceId": "plafrim-bora"})
		response["activityAttempts"] = append(arr(response["activityAttempts"]), Object{"activityId": id, "attempt": 1, "status": "completed", "runtimeSeconds": 300.0})
		response["handles"] = append(arr(response["handles"]), Object{"id": "training:" + id, "activityId": id, "status": "completed", "exitCode": 0, "artifacts": Object{"finalSnapshot": []any{Object{}, Object{}}}})
	}
	r := supplementRoots(c, p, request, response, "digest")
	row := rows(r["computeObservations"])[0]
	require.Equal(t, list("plafrim-bora"), row["allowedResourceIds"])
	require.Equal(t, 50.0, obj(row["wallSecondsPerReferenceSecond"])["median"])
	require.Empty(t, c["computeObservations"])
	rows(response["handles"])[0]["exitCode"] = nil
	bad(t, func() Object { return supplementRoots(c, p, request, response, "digest") })
}
func TestCalibrationSafetyAndProvenance(t *testing.T) {
	for _, name := range []string{"barrier_violation_count", "unreleased_lease_count", "cross_cloud_hpc_non_relay_count"} {
		t.Run(name, func(t *testing.T) {
			a := scientific(t, "docker-image-real-v2-results.json")
			obj(a["audit"])[name] = 1
			bad(t, func() Object { return calibration(a, "d", nil) })
		})
	}
	a := scientific(t, "docker-image-real-v2-results.json")
	arr(a["activities"])[1] = arr(a["activities"])[0]
	bad(t, func() Object { return calibration(a, "d", nil) })
	a = scientific(t, "docker-image-real-v2-results.json")
	p := scientific(t, "workspace-projection-v2.json")
	p["baselineRunId"] = "other"
	bad(t, func() Object { return calibration(a, str(p["auditedEvidenceSHA256"]), p) })
}
func TestSummaryAndPooledService(t *testing.T) {
	require.Equal(t, normal(Object{"samples": 4, "median": 2.5, "minimum": 1, "maximum": 4}), normal(summary([]any{4, 2, 1, 3})))
	require.Nil(t, summary([]any{nil}))
	for _, v := range []float64{math.Inf(1), math.NaN(), -1} {
		bad(t, func() Object { return Object{"summary": summary([]any{v})} })
	}
	rate := obj(pooledRate([]Object{{"archiveBytes": 100, "seconds": 10}, {"archiveBytes": 1000, "seconds": 20}, {"archiveBytes": nil, "seconds": 999}}, "archiveBytes"))
	require.Equal(t, 2, rate["samples"])
	require.InDelta(t, 1100.0/30, num(rate["bytesPerSecond"]), 1e-12)
	require.Nil(t, pooledRate(nil, "archiveBytes"))
}
func TestQueueSeparation(t *testing.T) {
	for _, mode := range []string{"pack", "upload", "receiver"} {
		t.Run(mode, func(t *testing.T) {
			a := scientific(t, "docker-image-real-v2-results.json")
			for _, r := range rows(a["routes"]) {
				switch mode {
				case "pack":
					if r["strategy"] == "gateway" {
						r["source_pack_execution_seconds"] = 10
						r["source_pack_ssh_channel_wait_seconds"] = 20
					}
				case "upload":
					if r["strategy"] == "gateway" {
						r["rsync_execution_seconds"] = 7
						r["ssh_channel_wait_seconds"] = 19
					}
				case "receiver":
					for _, p := range rows(r["phases"]) {
						if strings.HasSuffix(str(p["name"]), "receiver-verify-and-extract") {
							p["executionSeconds"] = 3
							p["sshChannelWaitSeconds"] = 12
							p["durationSeconds"] = 15
						}
					}
				}
			}
			c := calibration(a, "d", nil)
			seen := 0
			for _, p := range rows(c["materializationPhaseObservations"]) {
				if mode == "pack" && p["phase"] == "pack-and-receive" {
					require.Equal(t, 10.0, num(obj(p["seconds"])["median"]))
					require.Equal(t, 20.0, num(obj(p["sourceChannelWaitSeconds"])["median"]))
					seen++
				}
				if mode == "upload" && p["phase"] == "archive-upload" {
					require.Equal(t, 7.0, num(obj(p["seconds"])["median"]))
					require.Equal(t, 19.0, num(obj(p["phaseChannelWaitSeconds"])["median"]))
					seen++
				}
				if mode == "receiver" && strings.HasSuffix(str(p["phase"]), "receiver-verify-and-extract") {
					require.Equal(t, 3.0, num(obj(p["seconds"])["median"]))
					require.Equal(t, 12.0, num(obj(p["phaseChannelWaitSeconds"])["median"]))
					seen++
				}
			}
			require.Positive(t, seen)
		})
	}
}
func TestFreezeRejectsUnmeasuredAndMixedQueue(t *testing.T) {
	resources := Object{
		"relay": Object{"domain": "relay", "class": "relay"},
		"hpc":   Object{"domain": "hpc", "class": "hpc", "bulkServiceGroupId": "group", "bulkServiceSlots": 3, "bulkServiceSource": "fixture"},
		"micro": Object{"domain": "cloud", "class": "e2-micro"},
	}
	c := scientific(t, "execution-calibration-v2.json")
	bad(t, func() Object { return freezeMaterialization(c, "d", resources, "relay") })
	for _, p := range rows(c["materializationPhaseObservations"]) {
		if p["sourceClass"] == "hpc" && p["phase"] == "pack-and-receive" {
			p["secondsBases"] = list("source-command-execution-excludes-channel-queue")
			p["sourcePreflightServiceSeconds"] = Object{"samples": 2, "median": 0.4}
		}
		if p["targetClass"] == "hpc" && strings.HasSuffix(str(p["phase"]), "receiver-verify-and-extract") {
			p["secondsBases"] = list("phase-command-execution-excludes-channel-queue")
		}
	}
	m := freezeMaterialization(c, "d", resources, "relay")
	require.Len(t, rows(m["profiles"]), 7)
	for _, p := range rows(c["materializationPhaseObservations"]) {
		if p["sourceClass"] == "hpc" && p["phase"] == "pack-and-receive" {
			p["secondsBases"] = append(arr(p["secondsBases"]), "combined-phase-wall-includes-unseparated-service-queue")
			break
		}
	}
	bad(t, func() Object { return freezeMaterialization(c, "d", resources, "relay") })
	c["workspaceVolumeBasis"] = "output-only"
	bad(t, func() Object { return freezeMaterialization(c, "d", resources, "relay") })
}
func TestPooledRateAndMergeOverhead(t *testing.T) {
	r := Object{"pooledArchiveServiceRate": Object{"samples": 2, "bytesPerSecond": 36.5}, "legacy": Object{"samples": 2, "median": 999}}
	require.Equal(t, 36.5, serviceRate(r, "pooledArchiveServiceRate", "legacy"))
	delete(r, "pooledArchiveServiceRate")
	require.Equal(t, 999.0, serviceRate(r, "pooledArchiveServiceRate", "legacy"))
	r["pooledArchiveServiceRate"] = Object{"samples": 2, "bytesPerSecond": 0}
	bad(t, func() Object { return Object{"rate": serviceRate(r, "pooledArchiveServiceRate", "legacy")} })
	c := scientific(t, "execution-calibration-v2.json")
	for _, m := range rows(c["mergeRateObservations"]) {
		m["commandOverheadSeconds"] = Object{"samples": 3, "median": 2.5}
		m["channelWaitSeconds"] = Object{"samples": 3, "median": 30}
	}
	result := freezeMaterialization(c, "d", scientific(t, "materialization-resource-model.json"), "montage-relay-controller")
	for _, m := range rows(result["mergeProfiles"]) {
		require.Equal(t, 2.5, m["commandOverheadSeconds"])
		require.NotContains(t, m, "channelWaitSeconds")
	}
}
func TestWorkspaceFilesAndPackaging(t *testing.T) {
	for _, p := range []string{"/absolute", "a/../b", "a\\b", "a\x00b"} {
		bad(t, func() Object {
			return files([]any{Object{"path": p, "sizeBytes": 1, "checksum": "sha256:" + strings.Repeat("a", 64)}})
		})
	}
	bad(t, func() Object { return files([]any{Object{"path": "safe", "sizeBytes": 1, "checksum": "bad"}}) })
	seed := Object{"path": "runtime.hdr", "sizeBytes": 10, "contentId": "immutable"}
	input := Object{"path": "parent.fits", "sizeBytes": 100, "contentId": "parent"}
	output := Object{"path": "difference.fits", "sizeBytes": 50, "contentId": "output"}
	m := activityModel(Object{"parent.fits": input}, Object{"parent.fits": input, "runtime.hdr": seed, "difference.fits": output}, Object{"runtime.hdr": seed}, "audited")
	require.Equal(t, []any{output}, m["outputs"])
	require.Empty(t, obj(m["inputsByResourceType"])["hpc_machine"])
	changed := Object{"path": "runtime.hdr", "sizeBytes": 20, "contentId": "changed"}
	m = activityModel(Object{}, Object{"runtime.hdr": changed}, Object{"runtime.hdr": seed}, "audited")
	require.Equal(t, []any{changed}, m["outputs"])
	m = activityModel(Object{"runtime.hdr": seed}, Object{"runtime.hdr": seed}, Object{"runtime.hdr": seed}, "audited")
	require.Empty(t, m["outputs"])
	require.Empty(t, m["removedPaths"])
	bad(t, func() Object { return unionFiles([]Object{{"runtime.hdr": seed}, {"runtime.hdr": changed}}) })
}
func TestInputWorkAndHeaders(t *testing.T) {
	cards := ""
	for _, c := range []string{"SIMPLE  = T", "BITPIX  = 16", "NAXIS   = 2", "NAXIS1  = 2531", "NAXIS2  = 2536", "END"} {
		cards += fmt.Sprintf("%-80s", c)
	}
	feature := inputFeature(fitsHeader(strings.NewReader(cards)))
	require.InDelta(t, 6.418616, num(feature["computeWorkUnits"]), 1e-12)
	require.Equal(t, 16, feature["bitpix"])
	bad(t, func() Object { return fitsHeader(strings.NewReader("short")) })
	bad(t, func() Object { return inputFeature(Object{"NAXIS": 3}) })
	bad(t, func() Object { return inputFeature(Object{"NAXIS": 2, "NAXIS1": 0, "NAXIS2": 1}) })
	a := Object{"id": "workflow-mprojectid0000001", "name": "mprojectid0000001", "command": Object{"arguments": list("-c", "mProject -X 'input.fits' output.fits region.hdr")}, "simulation": Object{"durationSeconds": 999}}
	request := Object{"workflow": Object{"activities": []any{a}, "dependencies": []any{}}}
	inputs := Object{"sourceImageSHA256": "source", "inputs": []any{Object{"path": "input.fits", "band": "red", "unit": "input-megapixels", "computeWorkUnits": 6.0}}}
	work := workFeatures(request, inputs)
	obj(a["simulation"])["durationSeconds"] = 1
	require.Equal(t, work, workFeatures(request, inputs))
	obj(a["command"])["arguments"] = list("-c", "mProject missing.fits out hdr")
	bad(t, func() Object { return workFeatures(request, inputs) })
}
func auditFixture() (Object, Object, Object) {
	activities, assignments, tasks := []any{}, []any{}, []any{}
	for i := 1; i <= 58; i++ {
		id := fmt.Sprintf("activity-%d", i)
		activities = append(activities, Object{"id": id})
		assignments = append(assignments, Object{"activityId": id, "resourceId": "micro"})
		tasks = append(tasks, Object{"activityId": id, "attempt": 2, "status": "completed", "startedAt": 20, "metadata": Object{"address": "secret-address"}})
	}
	return Object{
		"workflow": Object{"activities": activities, "dependencies": []any{}},
		"plan":     Object{"predicted": Object{"makespanSeconds": 10, "secret": "never-export"}, "assignments": assignments},
	}, Object{
		"run":                  Object{"id": "run", "status": "completed"},
		"activityAttempts":     tasks,
		"artifactTransferRuns": []any{Object{"activityId": "activity-1", "status": "completed", "finishedAt": 19}},
		"workspaceLeases":      []any{Object{"released": true}},
	}, Object{
		"run": Object{},
	}
}
func TestAuditAllowlistBarrierLatestAndTimes(t *testing.T) {
	request, response, sim := auditFixture()
	response["activityAttempts"] = append(arr(response["activityAttempts"]), Object{"activityId": "activity-1", "attempt": 1, "status": "failed"})
	r := audit(request, response, sim, false)
	require.Len(t, rows(r["activities"]), 58)
	require.NotContains(t, string(encode(r)), "secret")
	require.Equal(t, "multiple_attempts", r["attempt_history"])
	obj(arr(response["artifactTransferRuns"])[0])["finishedAt"] = 21
	bad(t, func() Object { return audit(request, response, sim, false) })
	obj(arr(response["artifactTransferRuns"])[0])["finishedAt"] = 19
	obj(response["run"])["status"] = "running"
	bad(t, func() Object { return audit(request, response, sim, false) })
	request, response, sim = auditFixture()
	obj(response["run"])["startedAt"] = "2026-10-08T00:00:00Z"
	obj(response["run"])["finishedAt"] = "2026-10-08T00:01:00Z"
	obj(response["run"])["makespanSeconds"] = 100
	r = audit(request, response, sim, false)
	require.Equal(t, 60.0, r["real_api_run_timestamp_wall_seconds"])
}
func TestAuditStrictAndMicroOverlap(t *testing.T) {
	request, response, sim := auditFixture()
	handles := []any{}
	for _, t := range rows(response["activityAttempts"]) {
		t["attempt"] = 1
		handles = append(handles, Object{"id": "run:" + str(t["activityId"]), "runId": "run", "activityId": t["activityId"], "status": "completed", "exitCode": 0})
	}
	response["handles"] = handles
	response["cloudBilling"] = Object{"status": "estimated", "currency": "USD", "estimatedCost": 0.2}
	audit(request, response, sim, true)
	obj(handles[0])["exitCode"] = nil
	bad(t, func() Object { return audit(request, response, sim, true) })
	obj(handles[0])["exitCode"] = 0
	obj(handles[0])["id"] = "wrong"
	bad(t, func() Object { return audit(request, response, sim, true) })
	request, response, sim = auditFixture()
	request["resources"] = []any{Object{"id": "micro", "cpuCapacity": 1}}
	for i, t := range rows(response["activityAttempts"]) {
		t["startedAt"] = 20 + i*10
		t["finishedAt"] = 25 + i*10
	}
	audit(request, response, sim, false)
	obj(arr(response["activityAttempts"])[1])["startedAt"] = 24
	bad(t, func() Object { return audit(request, response, sim, false) })
}
func TestNativeTimingExactAttempt(t *testing.T) {
	request, response, sim := auditFixture()
	h := Object{"activityId": "activity-1", "startedAt": 20, "finishedAt": 25, "metadata": Object{"executionAttempt": 2, "terminalObservedAt": 28, "address": "never-export"}}
	response["handles"] = []any{h}
	r := audit(request, response, sim, false)
	require.Equal(t, 5.0, rows(r["activities"])[0]["validated_handle_runtime_seconds"])
	require.Equal(t, 3.0, rows(r["activities"])[0]["terminal_observation_delay_seconds"])
	obj(h["metadata"])["executionAttempt"] = 1
	require.Nil(t, rows(audit(request, response, sim, false)["activities"])[0]["validated_handle_runtime_seconds"])
	obj(h["metadata"])["executionAttempt"] = 2
	obj(h["metadata"])["terminalObservedAt"] = 24
	require.Nil(t, rows(audit(request, response, sim, false)["activities"])[0]["validated_handle_runtime_seconds"])
}
func TestFixedPlacementAndLocalComparison(t *testing.T) {
	observed := Object{
		"run_id":     "observed",
		"activities": []any{Object{"activity_id": "a", "resource_id": "vm"}, Object{"activity_id": "b", "resource_id": "vm"}},
		"routes": []any{
			Object{"producer_activity_id": "a", "consumer_activity_id": "b", "strategy": "direct-runtime", "network_bytes": 0, "phases": []any{Object{"name": "shared-conflict-check-copy-and-verify", "durationSeconds": 1000}}},
		},
		"real_api_reported": Object{"makespanSeconds": 100},
	}
	request := Object{"run": Object{"mode": "simulation"}, "plan": Object{"assignments": []any{Object{"activityId": "a", "resourceId": "vm"}, Object{"activityId": "b", "resourceId": "vm"}}}, "networkTopology": Object{"metadata": Object{}}}
	prepared := fixedRequest(request, Object{"source": "old"}, observed, "sim-only")
	require.Equal(t, normal(request["plan"]), normal(prepared["plan"]))
	require.NotContains(t, obj(request["run"]), "id")
	obj(request["run"])["mode"] = "real"
	bad(t, func() Object { return fixedRequest(request, Object{}, observed, "bad") })
	sim := Object{"transfers": []any{}, "executed": Object{"makespanSeconds": 90}}
	r := compare(observed, sim, "calibration-fit-not-independent-validation", Object{"source": "observed"})
	require.Equal(t, 0, r["routeMismatchCount"])
	require.Equal(t, -10.0, r["makespanErrorPercent"])
	require.Equal(t, false, r["accepted"])
	obj(arr(observed["routes"])[0])["network_bytes"] = 10
	r = compare(observed, sim, "original-frozen-forecast", Object{"source": "old"})
	require.Equal(t, 1, r["routeMismatchCount"])
	require.Equal(t, -100.0, r["networkBytesErrorPercent"])
	bad(t, func() Object { return compare(observed, Object{}, "unlabeled", Object{}) })
}
func TestSourceCapacityNotNICOrQueue(t *testing.T) {
	flows := []any{}
	for i := 0; i < 3; i++ {
		flows = append(flows, Object{
			"status": "completed",
			"route": Object{
				"strategy":                        "gateway",
				"sourceResourceId":                "hpc",
				"targetResourceId":                "montage-gcp-fog-e2-micro-02",
				"archiveBytes":                    100,
				"sourcePackExecutionSeconds":      10,
				"sourcePackSshChannelWaitSeconds": 999,
				"phases":                          []any{Object{"name": "pack-and-receive", "finishedAt": 20}},
			},
		})
	}
	c := sourceCapacity(Object{"artifactTransferRuns": flows}, "hpc", "e2-micro")
	require.Equal(t, 3, c["concurrentCommandsMaximum"])
	require.Equal(t, 10.0, c["busySeconds"])
	require.Equal(t, 30.0, c["durationWeightedMedianBytesPerSecond"])
	request := Object{
		"networkTopology": Object{
			"metadata": Object{"workspaceMaterializationModel": Object{"relayResourceId": "relay"}},
			"links":    []any{Object{"id": "physical", "sourceResourceId": "relay", "targetResourceId": "hpc", "bidirectional": true, "bandwidthBitsPerSecond": 8000}},
		},
	}
	r := applyCapacity(request, "hpc", c)
	links := rows(obj(r["networkTopology"])["links"])
	require.Equal(t, false, links[0]["bidirectional"])
	require.Equal(t, 8000.0, num(links[0]["bandwidthBitsPerSecond"]))
	require.Equal(t, 240.0, num(links[1]["bandwidthBitsPerSecond"]))
	require.Equal(t, true, rows(obj(request["networkTopology"])["links"])[0]["bidirectional"])
	bad(t, func() Object { return sourceCapacity(Object{"artifactTransferRuns": []any{}}, "hpc", "e2-micro") })
}
func TestCLIRejectsBadArgsAndWritesPrivateJSON(t *testing.T) {
	require.Error(t, Run(nil))
	require.Error(t, Run([]string{"unknown"}))
	dir := t.TempDir()
	out := filepath.Join(dir, "result.json")
	input := filepath.Join("..", "..", "experiments", "montage-micro-hpc-e2", "docker-image-real-v2-results.json")
	require.NoError(t, Run([]string{"build_montage_execution_calibration", input, out}))
	info, err := os.Stat(out)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	require.Error(t, Run([]string{"export_cloud_workspace", "bad", "bad", out}))
}
