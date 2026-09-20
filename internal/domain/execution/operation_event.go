package execution

import "time"

type WorkflowOperationEvent struct {
	ID                string         `json:"id"`
	ExecutionRunID    string         `json:"executionRunId"`
	ActivityID        string         `json:"activityId,omitempty"`
	OperationID       string         `json:"operationId"`
	ParentOperationID string         `json:"parentOperationId,omitempty"`
	TransferRunID     string         `json:"transferRunId,omitempty"`
	CommandID         string         `json:"commandId,omitempty"`
	Sequence          int64          `json:"sequence"`
	Level             string         `json:"level"`
	Category          string         `json:"category"`
	Phase             string         `json:"phase"`
	Message           string         `json:"message"`
	CommandSanitized  string         `json:"commandSanitized,omitempty"`
	ProgressBytes     int64          `json:"progressBytes,omitempty"`
	TotalBytes        int64          `json:"totalBytes,omitempty"`
	ThroughputBPS     float64        `json:"throughputBps,omitempty"`
	ExitCode          *int           `json:"exitCode,omitempty"`
	StdoutExcerpt     string         `json:"stdoutExcerpt,omitempty"`
	StderrExcerpt     string         `json:"stderrExcerpt,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	OccurredAt        time.Time      `json:"occurredAt"`
}
