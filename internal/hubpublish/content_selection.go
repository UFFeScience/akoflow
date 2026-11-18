package hubpublish

import (
	"encoding/json"
	"regexp"
	"strings"
)

var machineLabel = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _-]{0,47}$`)

type ContentSelection struct {
	MachineLabel string `json:"machineLabel,omitempty"`
	Logs         bool   `json:"logs"`
	Files        bool   `json:"files"`
	Metrics      bool   `json:"metrics"`
	Events       bool   `json:"events"`
}

func selectCapture(capture capturedExperiment, selection ContentSelection) (capturedExperiment, error) {
	label := strings.TrimSpace(selection.MachineLabel)
	if label != "" && !machineLabel.MatchString(label) {
		return capturedExperiment{}, failure(422, "invalid_machine_label", "Use up to 48 letters, numbers, spaces, hyphens or underscores for the public machine label.")
	}
	execution := capture.execution
	if environment, ok := execution["environmentSnapshot"].(publishedEnvironment); ok {
		environment.MachineLabel = label
		execution["environmentSnapshot"] = environment
	}
	if !selection.Logs {
		delete(execution, "processLogs")
	}
	if !selection.Files {
		for _, key := range []string{"files", "dataObjects", "dataLocations", "dataTransfers", "workspaces", "workspaceLeases", "artifactMaterializations", "artifactTransferRuns"} {
			delete(execution, key)
		}
	}
	if !selection.Metrics {
		delete(execution, "metricSummaries")
	}
	if !selection.Events {
		for _, key := range []string{"events", "operationEvents", "timeline"} {
			delete(execution, key)
		}
	}
	var plans []any
	for _, file := range capture.bundle.Files {
		if file.Path == "plans.json" {
			if err := json.Unmarshal([]byte(file.Content), &plans); err != nil {
				return capturedExperiment{}, err
			}
		}
	}
	return encodeExperiment(capture.workflow, plans, execution, capture.yaml, capture.bundle.Redactions)
}
