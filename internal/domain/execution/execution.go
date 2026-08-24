package execution

import (
	"time"

	"github.com/UFFeScience/akoflow/internal/domain/planning"
	"github.com/UFFeScience/akoflow/internal/domain/resource"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
)

type ExecutionRunStatus string
type TaskExecutionStatus string
type ActivityHandleStatus string
type ExecutionRunKind string

const (
	ExecutionRunWorkflow    ExecutionRunKind = "workflow"
	ExecutionRunInteractive ExecutionRunKind = "interactive"
	ExecutionRunStandalone  ExecutionRunKind = "standalone"
)

// Runtime timing metadata has one contract across real runtimes. SubmittedAt
// is when the control plane handed work to a runtime, StartedAt is when that
// runtime allocated execution, and ContainerStartedAt is when user code can
// begin inside its container. Adapters omit ContainerStartedAt only when they
// do not execute a container.
const (
	TimingSubmittedAt        = "submittedAt"
	TimingContainerStartedAt = "containerStartedAt"
)

const (
	ExecutionRunCreated   ExecutionRunStatus = "created"
	ExecutionRunRunning   ExecutionRunStatus = "running"
	ExecutionRunCompleted ExecutionRunStatus = "completed"
	ExecutionRunFailed    ExecutionRunStatus = "failed"

	TaskBlocked   TaskExecutionStatus = "blocked"
	TaskReady     TaskExecutionStatus = "ready"
	TaskPreparing TaskExecutionStatus = "preparing"
	TaskRunning   TaskExecutionStatus = "running"
	TaskCompleted TaskExecutionStatus = "completed"
	TaskFailed    TaskExecutionStatus = "failed"
	TaskCancelled TaskExecutionStatus = "cancelled"

	HandleStarting  ActivityHandleStatus = "starting"
	HandleRunning   ActivityHandleStatus = "running"
	HandleCompleted ActivityHandleStatus = "completed"
	HandleFailed    ActivityHandleStatus = "failed"
	HandleStopped   ActivityHandleStatus = "stopped"
)

type ExecutionRun struct {
	ID                     string                 `json:"id"`
	SchedulePlanID         string                 `json:"schedulePlanId"`
	Mode                   planning.ExecutionMode `json:"mode"`
	Seed                   int64                  `json:"seed"`
	Status                 ExecutionRunStatus     `json:"status"`
	EnvironmentSnapshotID  string                 `json:"environmentSnapshotId,omitempty"`
	MakespanSeconds        float64                `json:"makespanSeconds"`
	Cost                   float64                `json:"cost"`
	FailureReason          string                 `json:"failureReason,omitempty"`
	ActivityCount          int                    `json:"activityCount,omitempty"`
	CompletedActivityCount int                    `json:"completedActivityCount,omitempty"`
	Breakdown              ExecutionBreakdown     `json:"breakdown"`
	TransferredBytes       int64                  `json:"transferredBytes"`
	Kind                   ExecutionRunKind       `json:"kind"`
	Title                  string                 `json:"title,omitempty"`
	ResourceID             string                 `json:"resourceId,omitempty"`
	RuntimeID              string                 `json:"runtimeId,omitempty"`
	ConnectionID           string                 `json:"connectionId,omitempty"`
	ActorID                string                 `json:"actorId,omitempty"`
	Command                string                 `json:"command,omitempty"`
	StartedAt              *time.Time             `json:"startedAt,omitempty"`
	FinishedAt             *time.Time             `json:"finishedAt,omitempty"`
}

type ExecutionRunPage struct {
	Items    []ExecutionRun `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Total    int            `json:"total"`
	HasNext  bool           `json:"hasNext"`
}

type ExecutionBreakdown struct {
	ComputeSeconds      float64 `json:"computeSeconds"`
	TransferSeconds     float64 `json:"transferSeconds"`
	QueueSeconds        float64 `json:"queueSeconds"`
	InterferenceSeconds float64 `json:"interferenceSeconds"`
	OverheadSeconds     float64 `json:"overheadSeconds"`
}

type TaskExecution struct {
	ID                  string              `json:"id"`
	ExecutionRunID      string              `json:"executionRunId"`
	PlanAssignmentID    string              `json:"planAssignmentId"`
	ActivityID          string              `json:"activityId"`
	PlannedResourceID   string              `json:"plannedResourceId"`
	AllocatedResourceID string              `json:"allocatedResourceId,omitempty"`
	Attempt             int                 `json:"attempt"`
	Status              TaskExecutionStatus `json:"status"`
	ReadyAt             float64             `json:"readyAt"`
	DataReadyAt         float64             `json:"dataReadyAt"`
	QueuedAt            float64             `json:"queuedAt"`
	StartedAt           float64             `json:"startedAt"`
	FinishedAt          float64             `json:"finishedAt"`
	RuntimeSeconds      float64             `json:"runtimeSeconds"`
	QueueSeconds        float64             `json:"queueSeconds"`
	TransferSeconds     float64             `json:"transferSeconds"`
	TransferBytes       int64               `json:"transferBytes"`
	InterferenceSeconds float64             `json:"interferenceSeconds"`
	OverheadSeconds     float64             `json:"overheadSeconds"`
	Cost                float64             `json:"cost"`
	FailureReason       string              `json:"failureReason,omitempty"`
}

type ExecutionMetrics struct {
	MakespanSeconds     float64 `json:"makespanSeconds"`
	Cost                float64 `json:"cost"`
	ComputeSeconds      float64 `json:"computeSeconds"`
	TransferSeconds     float64 `json:"transferSeconds"`
	QueueSeconds        float64 `json:"queueSeconds"`
	InterferenceSeconds float64 `json:"interferenceSeconds"`
	OverheadSeconds     float64 `json:"overheadSeconds"`
	Feasible            bool    `json:"feasible"`
}

type ExecutionTrace struct {
	RunID     string                    `json:"runId"`
	PlanID    string                    `json:"planId"`
	Mode      planning.ExecutionMode    `json:"mode"`
	Predicted planning.PredictedMetrics `json:"predicted"`
	Executed  ExecutionMetrics          `json:"executed"`
	Tasks     []TaskExecution           `json:"tasks"`
	Transfers []DataTransfer            `json:"transfers"`
}

type DataTransfer struct {
	ID                 string  `json:"id"`
	ExecutionRunID     string  `json:"executionRunId"`
	ProducerActivityID string  `json:"producerActivityId"`
	ConsumerActivityID string  `json:"consumerActivityId"`
	SourceResourceID   string  `json:"sourceResourceId"`
	TargetResourceID   string  `json:"targetResourceId"`
	Bytes              int64   `json:"bytes"`
	StartedAt          float64 `json:"startedAt"`
	FinishedAt         float64 `json:"finishedAt"`
	DurationSeconds    float64 `json:"durationSeconds"`
	Cost               float64 `json:"cost"`
}

type ActivityExecutionContext struct {
	Run        ExecutionRun             `json:"run"`
	Workflow   workflow.WorkflowVersion `json:"workflow"`
	Activity   workflow.Activity        `json:"activity"`
	Assignment planning.PlanAssignment  `json:"assignment"`
	Resource   resource.Resource        `json:"resource"`
	RuntimeID  string                   `json:"runtimeId"`
	// Preparation is supplied by the orchestration/data plane when the
	// activity has executable or workspace requirements. Providers must not
	// start work until it is committed.
	Preparation *PreparationGate `json:"preparation,omitempty"`
}

// ActivityHandle is the runtime-independent identity of a started activity.
// ExternalID belongs to the adapter (PID, Kubernetes Job, Slurm Job or
// simulation event); callers never need to understand its format.
type ActivityHandle struct {
	ID         string               `json:"id"`
	RunID      string               `json:"runId"`
	ActivityID string               `json:"activityId"`
	ResourceID string               `json:"resourceId"`
	RuntimeID  string               `json:"runtimeId"`
	ExternalID string               `json:"externalId,omitempty"`
	Status     ActivityHandleStatus `json:"status"`
	Endpoints  []string             `json:"endpoints,omitempty"`
	StartedAt  float64              `json:"startedAt"`
	FinishedAt float64              `json:"finishedAt,omitempty"`
	ExitCode   *int                 `json:"exitCode,omitempty"`
	Failure    string               `json:"failure,omitempty"`
	Log        string               `json:"log,omitempty"`
	Artifacts  *ArtifactManifest    `json:"artifacts,omitempty"`
	Metadata   map[string]any       `json:"metadata,omitempty"`
}
