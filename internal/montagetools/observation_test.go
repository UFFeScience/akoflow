package montagetools

import "testing"

func TestObservationSensitivityExplicitlyPreservesNonValidationRole(t *testing.T) {
	model := Object{}
	metadata := Object{"workspaceMaterializationModel": model}
	request := Object{"NetworkTopology": Object{"metadata": metadata}}
	real := Object{"run": Object{"status": "completed"}, "handles": []any{Object{"status": "completed", "metadata": Object{"remoteWorkspaceSnapshotSeconds": 2.0, "artifactObservationBefore": `[{"size":100}]`}}}}
	observationSensitivity(request, real, "digest")
	profile := rows(model["inputObservationProfiles"])[0]
	if profile["targetClass"] != "e2-micro" || profile["secondsPerUniqueByte"] != .02 || obj(metadata["inputObservationSensitivity"])["role"] != "exploratory-wall-time-sensitivity-not-service-calibration" {
		t.Fatal("lost coefficient or provenance role")
	}
}
