package console

import "time"

type CommandStatus string

const (
	CommandRunning   CommandStatus = "running"
	CommandCompleted CommandStatus = "completed"
	CommandFailed    CommandStatus = "failed"
)

type Command struct {
	ID               string            `json:"id"`
	ResourceID       string            `json:"resourceId"`
	RuntimeID        string            `json:"runtimeId"`
	ConnectionID     string            `json:"connectionId"`
	ActorID          string            `json:"actorId,omitempty"`
	Command          string            `json:"command"`
	WorkingDirectory string            `json:"workingDirectory,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
	CPUCores         int               `json:"cpuCores,omitempty"`
	MemoryBytes      int64             `json:"memoryBytes,omitempty"`
	TimeoutSeconds   int               `json:"timeoutSeconds"`
	Status           CommandStatus     `json:"status"`
	Stdout           string            `json:"stdout,omitempty"`
	Stderr           string            `json:"stderr,omitempty"`
	ExitCode         *int              `json:"exitCode,omitempty"`
	ExternalID       string            `json:"externalId,omitempty"`
	Failure          string            `json:"failure,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	StartedAt        time.Time         `json:"startedAt"`
	FinishedAt       *time.Time        `json:"finishedAt,omitempty"`
}

type Request struct {
	ResourceID       string            `json:"resourceId"`
	ActorID          string            `json:"actorId,omitempty"`
	Command          string            `json:"command"`
	WorkingDirectory string            `json:"workingDirectory,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
	CPUCores         int               `json:"cpuCores,omitempty"`
	MemoryBytes      int64             `json:"memoryBytes,omitempty"`
	TimeoutSeconds   int               `json:"timeoutSeconds,omitempty"`
}

type SessionStatus string

const (
	SessionStarting  SessionStatus = "starting"
	SessionConnected SessionStatus = "connected"
	SessionClosed    SessionStatus = "closed"
	SessionFailed    SessionStatus = "failed"
)

type Session struct {
	ID           string        `json:"id"`
	ResourceID   string        `json:"resourceId"`
	RuntimeID    string        `json:"runtimeId"`
	ConnectionID string        `json:"connectionId"`
	ActorID      string        `json:"actorId,omitempty"`
	Status       SessionStatus `json:"status"`
	ExternalID   string        `json:"externalId,omitempty"`
	Failure      string        `json:"failure,omitempty"`
	CreatedAt    time.Time     `json:"createdAt"`
	ConnectedAt  *time.Time    `json:"connectedAt,omitempty"`
	FinishedAt   *time.Time    `json:"finishedAt,omitempty"`
}

type SessionRequest struct {
	ResourceID string `json:"resourceId"`
	ActorID    string `json:"actorId,omitempty"`
}
