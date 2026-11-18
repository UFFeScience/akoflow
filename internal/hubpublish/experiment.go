package hubpublish

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	apirequests "github.com/UFFeScience/akoflow/internal/api/requests"
	"github.com/UFFeScience/akoflow/internal/domain"
	"net/url"
	"time"
)

type experimentFile struct {
	Path      string `json:"path"`
	MediaType string `json:"mediaType"`
	SHA256    string `json:"sha256"`
	Content   string `json:"content"`
}
type experimentBundle struct {
	Format        string            `json:"format"`
	SchemaVersion string            `json:"schemaVersion"`
	ExportedAt    time.Time         `json:"exportedAt"`
	Producer      map[string]string `json:"producer"`
	Source        map[string]string `json:"source"`
	Files         []experimentFile  `json:"files"`
	Redactions    []string          `json:"redactions"`
}
type capturedExperiment struct {
	bundle    experimentBundle
	workflow  map[string]any
	execution map[string]any
	yaml      string
}

func digest(data []byte) string { value := sha256.Sum256(data); return hex.EncodeToString(value[:]) }
func (m *Manager) capture(ctx context.Context, runID string) (capturedExperiment, error) {
	var result capturedExperiment
	var execution map[string]any
	runPath := "/execution-runs/" + url.PathEscape(runID) + "/"
	if runID == "" {
		return result, failure(422, "run_required", "Choose a completed workflow execution.")
	}
	if err := m.readSource(ctx, runPath, &execution); err != nil {
		return result, err
	}
	run, _ := execution["run"].(map[string]any)
	planID, _ := run["schedulePlanId"].(string)
	status, _ := run["status"].(string)
	if run["id"] != runID || planID == "" || (status != "completed" && status != "failed" && status != "cancelled") {
		return result, failure(422, "unfinished_run", "Only finished workflow executions can be published.")
	}
	var context map[string]any
	if err := m.readSource(ctx, "/schedule-plans/"+url.PathEscape(planID)+"/context/?portable=1", &context); err != nil {
		return result, err
	}
	var metrics struct {
		Items []any `json:"items"`
	}
	if err := m.readSource(ctx, runPath+"activity-metrics/", &metrics); err != nil {
		return result, err
	}
	var after map[string]any
	if err := m.readSource(ctx, runPath+"?view=status", &after); err != nil {
		return result, err
	}
	beforeJSON, _ := json.Marshal(run)
	afterJSON, _ := json.Marshal(after["run"])
	if digest(beforeJSON) != digest(afterJSON) {
		return result, failure(409, "run_changed", "The execution changed during export. Review it again.")
	}
	return buildExperiment(context, execution, metrics.Items)
}
func buildExperiment(context, execution map[string]any, metrics []any) (capturedExperiment, error) {
	var result capturedExperiment
	raw := map[string]any{"context": context, "execution": execution, "metrics": metrics}
	cleaned, redactions, err := redactSnapshot(raw)
	if err != nil {
		return result, err
	}
	snapshot := cleaned.(map[string]any)
	context = snapshot["context"].(map[string]any)
	execution = snapshot["execution"].(map[string]any)
	workflow, _ := context["workflow"].(map[string]any)
	plan, _ := context["plan"].(map[string]any)
	version, _ := workflow["version"].(map[string]any)
	run, _ := execution["run"].(map[string]any)
	workflowID, _ := workflow["id"].(string)
	versionID, _ := version["id"].(string)
	if workflowID == "" || versionID == "" ||
		plan["workflowVersionId"] != version["id"] ||
		run["schedulePlanId"] != plan["id"] ||
		run["workflowVersionId"] != nil && run["workflowVersionId"] != "" && run["workflowVersionId"] != version["id"] {
		return result, failure(422, "unrelated_snapshot", "The execution does not match its persisted workflow version.")
	}
	data, _ := json.Marshal(workflow)
	var definition domain.WorkflowDefinition
	if json.Unmarshal(data, &definition) != nil {
		return result, failure(422, "invalid_workflow", "The captured workflow is invalid.")
	}
	yaml, err := apirequests.FromDomain(definition).YAML()
	if err != nil || len(yaml) > 8<<20 {
		return result, failure(422, "invalid_workflow", "The captured workflow YAML is invalid or too large.")
	}
	environment := executionEnvironment(context, execution)
	execution = exportExecution(execution, snapshot["metrics"])
	execution["environmentSnapshot"] = environment
	plans := []any{map[string]any{"plan": plan, "context": context}}
	return encodeExperiment(workflow, plans, execution, string(yaml), redactions)
}
func exportExecution(detail map[string]any, metrics any) map[string]any {
	keys := []string{"run", "activities",
		"activityAttempts", "dataTransfers", "timeline", "cloudBilling",
		"artifactTransferRuns", "dataObjects", "dataLocations", "workspaces",
		"workspaceLeases", "artifactMaterializations", "infrastructureRuns", "events", "operationEvents"}
	result := map[string]any{}
	for _, key := range keys {
		if value, ok := detail[key]; ok {
			result[key] = value
		}
	}
	logs := []any{}
	handles, _ := detail["handles"].([]any)
	for _, handle := range handles {
		source, _ := handle.(map[string]any)
		record := map[string]any{}
		for _, key := range []string{"activityId", "attempt", "runtime", "status", "startedAt", "finishedAt", "exitCode", "log"} {
			if value, ok := source[key]; ok {
				record[key] = value
			}
		}
		logs = append(logs, record)
	}
	result["processLogs"] = logs
	result["metricSummaries"] = metrics
	if result["metricSummaries"] == nil {
		result["metricSummaries"] = []any{}
	}
	result["files"] = detail["dataObjects"]
	return result
}
func encodeExperiment(workflow map[string]any, plans []any, execution map[string]any, yaml string, redactions []string) (capturedExperiment, error) {
	version := workflow["version"].(map[string]any)
	bundle := experimentBundle{
		Format:        "akoflow.experiment",
		SchemaVersion: "1.0",
		ExportedAt:    time.Now().UTC(),
		Producer:      map[string]string{"name": "AkôFlow daemon", "version": "1.0"},
		Source: map[string]string{
			"workflowId":        workflow["id"].(string),
			"workflowVersionId": version["id"].(string),
		},
		Redactions: redactions,
	}
	files := []struct {
		path  string
		value any
	}{{"workflow.yaml", yaml}, {"workflow.json", workflow}, {"plans.json", plans}, {"executions.json", []any{execution}}}
	for _, file := range files {
		content := yaml
		mediaType := "application/yaml"
		if file.path != "workflow.yaml" {
			data, err := json.Marshal(file.value)
			if err != nil {
				return capturedExperiment{}, err
			}
			content = string(data)
			mediaType = "application/json"
		}
		bundle.Files = append(bundle.Files, experimentFile{Path: file.path, MediaType: mediaType, SHA256: digest([]byte(content)), Content: content})
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		return capturedExperiment{}, err
	}
	if len(data) > packageLimit {
		return capturedExperiment{}, failure(422, "package_too_large", "This capture exceeds 8 MiB. No logs or observations were truncated.")
	}
	return capturedExperiment{bundle: bundle, workflow: workflow, execution: execution, yaml: yaml}, nil
}
