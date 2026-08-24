package audit

import "time"

type Outcome string

const (
	OutcomeStarted   Outcome = "started"
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeFailed    Outcome = "failed"
)

type Event struct {
	ID            string         `json:"id"`
	EventType     string         `json:"eventType"`
	ActorID       string         `json:"actorId,omitempty"`
	ActorType     string         `json:"actorType,omitempty"`
	EnvironmentID string         `json:"environmentId,omitempty"`
	ResourceID    string         `json:"resourceId,omitempty"`
	ConnectionID  string         `json:"connectionId,omitempty"`
	RuntimeID     string         `json:"runtimeId,omitempty"`
	SessionID     string         `json:"sessionId,omitempty"`
	ExecutionID   string         `json:"executionId,omitempty"`
	ExternalID    string         `json:"externalId,omitempty"`
	Outcome       Outcome        `json:"outcome"`
	Summary       string         `json:"summary"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	OccurredAt    time.Time      `json:"occurredAt"`
}

type Filter struct {
	EventType     string
	EnvironmentID string
	ResourceID    string
	ConnectionID  string
	SessionID     string
	ExecutionID   string
	Outcome       Outcome
	Limit         int
}
