package events

import "time"

const (
	ExecutionStarted   = "execution.run.started"
	ExecutionCompleted = "execution.run.completed"
	ExecutionFailed    = "execution.run.failed"
	ActivityStarted    = "activity.started"
	ActivityCompleted  = "activity.completed"
	ActivityFailed     = "activity.failed"
)

type Event struct {
	ID            string         `json:"id"`
	Type          string         `json:"eventType"`
	AggregateType string         `json:"aggregateType"`
	AggregateID   string         `json:"aggregateId"`
	Payload       map[string]any `json:"payload,omitempty"`
	OccurredAt    time.Time      `json:"occurredAt"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}
