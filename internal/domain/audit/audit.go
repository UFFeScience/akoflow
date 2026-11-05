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
	Category      string
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

type FacetValue struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

type FacetPage struct {
	Values  []FacetValue `json:"values"`
	HasMore bool         `json:"hasMore"`
}

type TimeBucket struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int64     `json:"count"`
}

type Summary struct {
	FacetMore map[string]bool         `json:"facetMore,omitempty"`
	Total     int64                   `json:"total"`
	From      time.Time               `json:"from"`
	To        time.Time               `json:"to"`
	Buckets   []TimeBucket            `json:"buckets"`
	Facets    map[string][]FacetValue `json:"facets"`
}
