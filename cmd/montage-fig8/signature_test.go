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
