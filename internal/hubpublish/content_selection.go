package hubpublish

import "encoding/json"

type ContentSelection struct {
	Logs    bool `json:"logs"`
	Files   bool `json:"files"`
	Metrics bool `json:"metrics"`
	Events  bool `json:"events"`
}

func selectCapture(capture capturedExperiment, selection ContentSelection) (capturedExperiment, error) {
	execution := capture.execution
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
