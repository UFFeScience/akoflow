package hubpublish

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestHubCaptureIncludesOnlySelectedExecutionAndHistoricalWorkflow(t *testing.T) {
	run := map[string]any{"id": "selected", "schedulePlanId": "plan", "workflowVersionId": "historical", "status": "completed", "makespanSeconds": 2.5, "cost": 0}
	workflow := map[string]any{"id": "workflow", "name": "Historical workflow", "version": map[string]any{"id": "historical", "activities": []any{}}}
	contextData := map[string]any{
		"workflow": workflow, "plan": map[string]any{"id": "plan", "workflowVersionId": "historical"},
	}
	manager := testManager(t, func(w http.ResponseWriter, r *http.Request) { t.Error("capture must not call Hub") })
	manager.options.Source = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var value any
		switch r.URL.Path {
		case "/akoflow-api/execution-runs/selected/":
			value = map[string]any{"run": run}
			if r.URL.RawQuery == "" {
				value = map[string]any{
					"run": run, "activities": []any{map[string]any{"id": "activity", "cpuSeconds": 4.5}},
					"handles":     []any{map[string]any{"activityId": "activity", "log": "captured output", "token": "secret-token", "opaque": "runtime-only"}},
					"dataObjects": []any{map[string]any{"id": "file", "logicalPath": "results/output.csv"}},
				}
			}
		case "/akoflow-api/schedule-plans/plan/context/":
			value = contextData
		case "/akoflow-api/execution-runs/selected/activity-metrics/":
			value = map[string]any{"items": []any{map[string]any{"cpuSeconds": 4.5}}}
		default:
			t.Errorf("unexpected source read: %s", r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(value)
	})
	capture, err := manager.capture(context.Background(), "selected")
	if err != nil {
		t.Fatal(err)
	}
	if capture.bundle.Source["workflowVersionId"] != "historical" {
		t.Fatal("current workflow replaced historical version")
	}
	if len(capture.bundle.Files) != 4 {
		t.Fatal("incomplete package")
	}
	for _, file := range capture.bundle.Files {
		if digest([]byte(file.Content)) != file.SHA256 {
			t.Fatal("invalid digest")
		}
		if strings.Contains(file.Content, "secret-token") || strings.Contains(file.Content, "runtime-only") {
			t.Fatal("runtime secrets exported")
		}
		if file.Path == "executions.json" {
			var executions []map[string]any
			if json.Unmarshal([]byte(file.Content), &executions) != nil || len(executions) != 1 {
				t.Fatal("exported unrelated executions")
			}
			if !strings.Contains(file.Content, "captured output") || !strings.Contains(file.Content, "results/output.csv") || !strings.Contains(file.Content, "4.5") {
				t.Fatal("lost logs, file references or scientific observations")
			}
		}
	}
	run["status"] = "running"
	if _, err := manager.capture(context.Background(), "selected"); err == nil {
		t.Fatal("running execution exported")
	}
}
func TestHubCaptureRejectsUnrelatedPlanAndOversizedPackage(t *testing.T) {
	workflow := map[string]any{"id": "workflow", "name": "Workflow", "version": map[string]any{"id": "version"}}
	contextData := map[string]any{"workflow": workflow, "plan": map[string]any{"id": "plan", "workflowVersionId": "different"}}
	execution := map[string]any{"run": map[string]any{"id": "run", "schedulePlanId": "plan"}}
	if _, err := buildExperiment(contextData, execution, nil); err == nil {
		t.Fatal("unrelated plan accepted")
	}
	execution["log"] = strings.Repeat("a", packageLimit)
	if _, err := encodeExperiment(workflow, []any{}, execution, "name: Workflow", nil); err == nil {
		t.Fatal("oversized capture silently truncated")
	}
}

func TestExperimentPackageCarriesArtifactInspectionInYAMLAndSnapshot(t *testing.T) {
	inspection := map[string]any{"schemaVersion": 1, "data": map[string]any{"format": "sif", "reference": "/apps/tool.sif", "sifPartitions": "1 |1 |NONE |100-200 |Def.FILE"}}
	activity := map[string]any{"id": "activity", "name": "Activity", "command": map[string]any{"executable": map[string]any{"source": map[string]any{"type": "local-file", "path": "/apps/tool.sif", "format": "sif", "inspection": inspection}}}}
	workflow := map[string]any{"id": "workflow", "name": "Workflow", "version": map[string]any{"id": "version", "activities": []any{activity}}}
	contextData := map[string]any{"workflow": workflow, "plan": map[string]any{"id": "plan", "workflowVersionId": "version"}}
	execution := map[string]any{"run": map[string]any{"id": "run", "schedulePlanId": "plan", "workflowVersionId": "version"}}
	capture, err := buildExperiment(contextData, execution, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range capture.bundle.Files {
		if file.Path == "workflow.yaml" || file.Path == "workflow.json" || file.Path == "plans.json" {
			if !strings.Contains(file.Content, "sifPartitions") || !strings.Contains(file.Content, "Def.FILE") {
				t.Fatalf("%s lost inspected image data", file.Path)
			}
		}
	}
}
