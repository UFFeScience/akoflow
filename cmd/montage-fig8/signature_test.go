package main

import "testing"

func TestSignatureIgnoresLabelsButRejectsChangedExecution(t *testing.T) {
	req := obj{"Resources": []any{obj{"id": "r", "bootOverheadSeconds": 0., "containerOverheadSeconds": 0., "metadata": obj{}}}, "Workflow": obj{"id": "wf"}}
	plan := obj{"id": "a", "algorithm": "heft", "assignments": []any{obj{"activityId": "a", "resourceId": "r", "coreId": "c", "orderOnResource": 0., "predictedStartAt": 0., "predictedRuntimeSeconds": 2., "predictedFinishAt": 2.}}}
	original, e := physicalSignature(req, plan)
	if e != nil {
		t.Fatal(e)
	}
	plan["id"] = "b"
	plan["algorithm"] = "moheft"
	got, e := physicalSignature(req, plan)
	if e != nil || got != original {
		t.Fatal("scheduler labels changed physical signature", e)
	}
	plan["assignments"].([]any)[0].(map[string]any)["orderOnResource"] = 1.
	changed, _ := physicalSignature(req, plan)
	if changed == original {
		t.Fatal("changed lane order reused")
	}
}

func TestSignatureIncludesLifecycleTimingButNotActionLabels(t *testing.T) {
	req := obj{"Resources": []any{}}
	plan := obj{"assignments": []any{}, "lifecycleActions": []any{
		obj{"id": "provision-a", "action": "provision", "capacityTargetId": "vm", "earliestStart": 0.},
		obj{"id": "validate-a", "action": "validate", "capacityTargetId": "vm", "earliestStart": 90., "dependsOn": []any{"provision-a"}},
	}}
	original, err := physicalSignature(req, plan)
	if err != nil {
		t.Fatal(err)
	}
	actions := plan["lifecycleActions"].([]any)
	actions[0].(map[string]any)["id"] = "provision-b"
	actions[1].(map[string]any)["dependsOn"] = []any{"provision-b"}
	renamed, _ := physicalSignature(req, plan)
	if original != renamed {
		t.Fatal("action labels changed signature")
	}
	actions[1].(map[string]any)["earliestStart"] = 100.
	changed, _ := physicalSignature(req, plan)
	if original == changed {
		t.Fatal("changed provisioning timing was reused")
	}
}
