package montagetools

import "testing"

func TestTransferWallSensitivityFitsStagesWithoutSharedBarrierDuplication(t *testing.T) {
	profile := Object{"strategy": "direct-runtime", "sourceClass": "e2-micro", "targetClass": "e2-micro"}
	request := Object{"NetworkTopology": Object{"metadata": Object{"workspaceMaterializationModel": Object{"profiles": []any{profile}, "resources": Object{"micro": Object{"bulkServiceGroupId": "old", "bulkServiceSlots": 3, "bulkServiceSource": "old"}}}}}}
	real := Object{"run": Object{"status": "completed"}, "workspaces": []any{Object{"activityId": "p", "sizeBytes": 100.0}}, "artifactTransferRuns": []any{Object{"status": "completed", "strategy": "direct-runtime", "startedAt": 1.0, "logicalBytes": 1.0, "route": Object{"producerActivityId": "p", "consumerActivityId": "c", "sourceResourceId": "montage-gcp-fog-e2-micro-01", "targetResourceId": "montage-gcp-fog-e2-micro-02", "archiveBytes": 50.0, "phases": []any{
		Object{"name": "peer-snapshot-acquire", "durationSeconds": 12.0, "sshChannelWaitSeconds": 2.0}, Object{"name": "peer-private-upload", "durationSeconds": 5.0}, Object{"name": "peer-receiver-verify-and-extract", "durationSeconds": 2.0}, Object{"name": "shared-preparation-barrier", "durationSeconds": 1000.0},
	}}}}}
	result := transferWallSensitivity(request, real, "digest")
	model := obj(obj(obj(result["NetworkTopology"])["metadata"])["workspaceMaterializationModel"])
	fitted := obj(arr(model["profiles"])[0])
	if num(fitted["snapshotSecondsPerByte"]) != .1 || num(fitted["uploadArchiveBytesPerSecond"]) != 10 || num(fitted["receiverSecondsPerByte"]) != .02 {
		t.Fatal("wrong phase fit or duplicated barrier", fitted)
	}
	if obj(obj(model["resources"])["micro"])["bulkServiceGroupId"] != nil {
		t.Fatal("admission double charged")
	}
	if profile["snapshotSecondsPerByte"] != nil {
		t.Fatal("source forecast mutated")
	}
}
