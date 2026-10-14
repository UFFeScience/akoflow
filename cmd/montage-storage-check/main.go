// Offline storage bound diagnostic. No API submission or filesystem deletion.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain/storagebudget"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		panic("request path required")
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var r ports.ExecutionRequest
	if err = json.Unmarshal(b, &r); err != nil {
		panic(err)
	}
	for i := range r.Resources {
		if string(r.Resources[i].Type) != "hpc_machine" {
			continue
		}
		m := r.Resources[i].Metadata
		decision, _ := m["workspaceStorageBudgetDecision"].(map[string]any)
		if previous := decision["previousBudget"]; previous != nil {
			m["workspaceStorageBudget"] = previous
		}
		m["workspaceStorageLifecycle"] = "consumer-barrier-v1"
	}
	t := []storagebudget.Timing{}
	for _, a := range r.Plan.Assignments {
		t = append(t, storagebudget.Timing{ActivityID: a.ActivityID, ResourceID: a.ResourceID, Start: a.PredictedStartAt, Finish: a.PredictedFinishAt})
	}
	rows, missing, err := storagebudget.CheckScheduled(r.Workflow, r.Resources, t)
	result := map[string]any{"reservations": rows, "missing": missing}
	types := map[string]string{}
	for _, a := range r.Plan.Assignments {
		for _, resource := range r.Resources {
			if resource.ID == a.ResourceID {
				types[a.ActivityID] = string(resource.Type)
			}
		}
	}
	projected, projectionErr := workflow.ProjectWorkspaceDependenciesForResourceTypes(r.Workflow, types)
	if projectionErr == nil {
		volumes := []map[string]any{}
		for _, a := range projected.Activities {
			volumes = append(volumes, map[string]any{"activityId": a.ID, "volumes": a.Metadata["workspaceProjectionVolumes"]})
		}
		result["projectedVolumes"] = volumes
	}
	if err != nil {
		result["error"] = err.Error()
	}
	b, _ = json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))
}
