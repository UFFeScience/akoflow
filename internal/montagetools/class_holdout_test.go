package montagetools

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestPreparedHoldoutIsSimulationOnlyAndTopologicallySerialized(t *testing.T) {
	r, _ := stageFixture()
	rid := "montage-gcp-fog-e2-micro-01"
	for _, a := range rows(obj(r["Workflow"])["activities"]) {
		a["activityTypeId"] = "fixture"
		a["simulation"] = Object{"durationSeconds": 1}
	}
	rows(r["Resources"])[0]["computeSpeedup"] = 1
	r["ActivityProfiles"] = []any{Object{"id": "measured", "activityTypeId": "fixture", "resourceId": rid, "runtimeSeconds": 1, "modelVersion": "reference-work-v1", "sampleSize": 1}}
	p := Object{"targetClass": "e2-micro", "source": "fixture", "samples": 1}
	cal := Object{"NetworkTopology": Object{"metadata": Object{"runtimeStageCalibration": Object{"targetClass": "e2-micro", "fixedLaunchAndRuntimeOverheadSeconds": 2}, "workspaceMaterializationModel": Object{"mergeProfiles": []any{p}, "inputObservationProfiles": []any{p}, "publicationProfiles": []any{p}}}}}
	out := prepareClassHoldout(r, cal, rid, "montage-holdout-sim", "montage-holdout-plan")
	require.Equal(t, "simulation", obj(out["Run"])["mode"])
	require.Equal(t, false, obj(obj(out["Plan"])["metadata"])["realExecutionAuthorized"])
	assignments := rows(obj(out["Plan"])["assignments"])
	require.Equal(t, "new-rootid0000001", assignments[0]["activityId"])
	require.Equal(t, "new-childid0000002", assignments[1]["activityId"])
	require.Equal(t, assignments[0]["coreId"], assignments[1]["coreId"])
	require.NotContains(t, obj(rows(r["Resources"])[0]["metadata"]), "maximumConcurrentActivities")
}

func TestNativeVerificationRejectsRealBeforeDockerOrOutputCreation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "request.json")
	b, _ := json.Marshal(Object{"Run": Object{"mode": "real"}})
	require.NoError(t, os.WriteFile(path, b, 0600))
	out := filepath.Join(dir, "output")
	require.Panics(t, func() { verifyNativeCase(path, "unused", "/case/unused", out) })
	_, err := os.Stat(out)
	require.True(t, os.IsNotExist(err))
}
