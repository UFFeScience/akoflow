package main

import (
	"fmt"
	"testing"
)

func TestPhysicalAuditRejectsWrongNodePlacementAndExitDespiteCompleteCounters(t *testing.T) {
	assignments, handles, attempts := []any{}, []any{}, []any{}
	for i := range 58 {
		id := fmt.Sprintf("activity-%d", i)
		assignments = append(assignments, obj{"activityId": id, "resourceId": "plafrim-resource"})
		handles = append(handles, obj{"activityId": id, "resourceId": "plafrim-resource", "status": "completed", "exitCode": 0., "metadata": obj{"allocatedNode": "diablo03", "eligibleNodeFallback": false}})
		attempts = append(attempts, obj{})
	}
	plan := obj{"assignments": assignments}
	monitor := obj{"handles": handles, "activityAttempts": attempts}
	if err := auditPhysicalHandles(monitor, plan); err != nil {
		t.Fatal(err)
	}
	for name, poison := range map[string]func(obj){
		"wrong node":          func(h obj) { h["metadata"].(map[string]any)["allocatedNode"] = "diablo05" },
		"fallback":            func(h obj) { h["metadata"].(map[string]any)["eligibleNodeFallback"] = true },
		"wrong placement":     func(h obj) { h["resourceId"] = "other-resource" },
		"nonzero exit":        func(h obj) { h["exitCode"] = 1. },
		"duplicated activity": func(h obj) { h["activityId"] = "activity-1" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := clone(monitor)
			poison(bad["handles"].([]any)[0].(map[string]any))
			if auditPhysicalHandles(bad, plan) == nil {
				t.Fatal("accepted corrupted physical execution")
			}
		})
	}
}
