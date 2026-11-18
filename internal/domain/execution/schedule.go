package execution

import "time"

type Schedule struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	WorkflowVersionID string           `json:"workflowVersionId"`
	PlanID            string           `json:"planId"`
	Frequency         string           `json:"frequency"`
	LocalTime         string           `json:"localTime"`
	Weekday           int              `json:"weekday"`
	TimeZone          string           `json:"timeZone"`
	MissedPolicy      string           `json:"missedPolicy"`
	OverlapPolicy     string           `json:"overlapPolicy"`
	Paused            bool             `json:"paused"`
	NextAt            time.Time        `json:"nextAt"`
	LastOccurrence    *ScheduleOutcome `json:"lastOccurrence,omitempty"`
	LastLaunch        *ScheduleOutcome `json:"lastLaunch,omitempty"`
}

type ScheduleOutcome struct {
	ScheduledAt time.Time          `json:"scheduledAt"`
	HandledAt   time.Time          `json:"handledAt"`
	Outcome     string             `json:"outcome"`
	Detail      string             `json:"detail,omitempty"`
	RunID       string             `json:"runId,omitempty"`
	RunStatus   ExecutionRunStatus `json:"runStatus,omitempty"`
}
