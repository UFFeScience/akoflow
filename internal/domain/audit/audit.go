package audit

import "time"

type Outcome string

const (
	OutcomeStarted   Outcome = "started"
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeFailed    Outcome = "failed"
)

type Event struct {
	RequestID     string `json:"requestId,omitempty"`
	CorrelationID string `json:"correlationId,omitempty"`
	Transport     string `json:"transport,omitempty"`
	Source        string `json:"source,omitempty"`
	Method        string `json:"method,omitempty"`
	Route         string `json:"route,omitempty"`
	Status        int    `json:"status,omitempty"`
	DurationMS    int64  `json:"durationMs,omitempty"`
	AuthOutcome   string `json:"authOutcome,omitempty"`
	ActivityID    string `json:"activityId,omitempty"`
	Attempt       int    `json:"attempt,omitempty"`
	OperationID   string `json:"operationId,omitempty"`

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
	RequestID     string
	CorrelationID string
	ActorID       string
	Method        string
	Route         string
	Status        int
	Transport     string
	RuntimeID     string
	ActivityID    string
	OperationID   string
	Before        string
	From          string
	To            string

	EventType     string
	EnvironmentID string
	ResourceID    string
	ConnectionID  string
	SessionID     string
	ExecutionID   string
	Outcome       Outcome
	Limit         int
}
