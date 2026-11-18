package planning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
)

// ScheduleFingerprint identifies the runnable allocation independently of its generating algorithm.
func ScheduleFingerprint(plan SchedulePlan, predicted PredictedMetrics) (string, error) {
	assignments := slices.Clone(plan.Assignments)
	for i := range assignments {
		assignment := &assignments[i]
		runtime, _ := assignment.Metadata["runtimeId"].(string)
		assignment.ID, assignment.PlanID, assignment.Metadata = "", "", nil
		if runtime != "" {
			assignment.Metadata = map[string]any{"runtimeId": runtime}
		}
	}
	slices.SortFunc(assignments, func(a, b PlanAssignment) int {
		return strings.Compare(a.ActivityID, b.ActivityID)
	})
	actions := slices.Clone(plan.LifecycleActions)
	for i := range actions {
		actions[i].SchedulePlanID = ""
	}
	signature := struct {
		Workflow, Scope, Topology string
		Assignments               []PlanAssignment
		Actions                   []PlannedLifecycleAction
		Predicted                 PredictedMetrics
	}{plan.WorkflowVersionID, plan.ExecutionScopeID, plan.NetworkTopologyID, assignments, actions, predicted}
	data, err := json.Marshal(signature)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
