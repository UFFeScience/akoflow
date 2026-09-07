package planning

import (
	"time"

	"github.com/UFFeScience/akoflow/internal/domain/environment"
	"github.com/UFFeScience/akoflow/internal/domain/resource"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
)

type PlanningSource string
type ExecutionMode string
type PlanningStatus string

const (
	PlanningSourcePlugin     PlanningSource = "plugin"
	PlanningSourceManual     PlanningSource = "manual"
	PlanningSourceImported   PlanningSource = "imported"
	ExecutionModeReal        ExecutionMode  = "real"
	ExecutionModeSimulation  ExecutionMode  = "simulation"
	ExecutionModeInteractive ExecutionMode  = "interactive"
)

const (
	PlanningStatusQueued    PlanningStatus = "queued"
	PlanningStatusRunning   PlanningStatus = "running"
	PlanningStatusCompleted PlanningStatus = "completed"
	PlanningStatusFailed    PlanningStatus = "failed"
	PlanningStatusCancelled PlanningStatus = "cancelled"
)

type PredictedMetrics struct {
	MakespanSeconds float64 `json:"makespanSeconds"`
	Cost            float64 `json:"cost"`
	Feasible        bool    `json:"feasible"`
}

type SchedulePlan struct {
	ID                string           `json:"id"`
	WorkflowVersionID string           `json:"workflowVersionId"`
	ExecutionScopeID  string           `json:"executionScopeId"`
	NetworkTopologyID string           `json:"networkTopologyId,omitempty"`
	Source            PlanningSource   `json:"source"`
	Algorithm         string           `json:"algorithm"`
	AlgorithmVersion  string           `json:"algorithmVersion,omitempty"`
	Objective         string           `json:"objective,omitempty"`
	DeadlineSeconds   float64          `json:"deadlineSeconds"`
	Budget            float64          `json:"budget"`
	Predicted         PredictedMetrics `json:"predicted"`
	Assignments       []PlanAssignment `json:"assignments"`
	AssignmentCount   int              `json:"assignmentCount,omitempty"`
	Metadata          map[string]any   `json:"metadata,omitempty"`
}

type PlanAssignment struct {
	ID                       string         `json:"id"`
	PlanID                   string         `json:"planId"`
	ActivityID               string         `json:"activityId"`
	ResourceID               string         `json:"resourceId"`
	CoreID                   string         `json:"coreId,omitempty"`
	SlotID                   string         `json:"slotId,omitempty"`
	OrderOnResource          int            `json:"orderOnResource"`
	Priority                 int            `json:"priority"`
	PredictedReadyAt         float64        `json:"predictedReadyAt"`
	PredictedStartAt         float64        `json:"predictedStartAt"`
	PredictedFinishAt        float64        `json:"predictedFinishAt"`
	PredictedRuntimeSeconds  float64        `json:"predictedRuntimeSeconds"`
	PredictedTransferSeconds float64        `json:"predictedTransferSeconds"`
	PredictedCost            float64        `json:"predictedCost"`
	Metadata                 map[string]any `json:"metadata,omitempty"`
}

type PlannedLifecycleAction struct {
	ID               string         `json:"id"`
	SchedulePlanID   string         `json:"schedulePlanId"`
	CapacityTargetID string         `json:"capacityTargetId"`
	CloudInstanceID  string         `json:"cloudInstanceId,omitempty"`
	Action           string         `json:"action"`
	EarliestStart    float64        `json:"earliestStart"`
	ExpectedDuration float64        `json:"expectedDuration"`
	DependsOn        []string       `json:"dependsOn,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type PlanningRequest struct {
	Workflow         workflow.WorkflowVersion           `json:"workflow"`
	ExecutionScope   environment.ExecutionScope         `json:"executionScope"`
	Environments     []environment.EnvironmentVersion   `json:"environments"`
	Resources        []resource.Resource                `json:"resources"`
	NetworkTopology  resource.NetworkTopology           `json:"networkTopology"`
	ActivityProfiles []workflow.ActivityResourceProfile `json:"activityProfiles"`
	DeadlineSeconds  float64                            `json:"deadlineSeconds"`
	Budget           float64                            `json:"budget"`
	Interference     *InterferenceMatrix                `json:"interference,omitempty"`
}

// InterferenceMatrix is an immutable planning-session input. Entries are
// directed: PriorityWeight is the affected activity's relative CPU priority
// while InterferingActivityID overlaps it on the same resource. A missing pair
// keeps the SimGrid default priority (1).
type InterferenceMatrix struct {
	SchemaVersion string              `json:"schemaVersion"`
	Model         string              `json:"model"`
	Aggregation   string              `json:"aggregation"`
	Entries       []InterferenceEntry `json:"entries"`
}

type InterferenceEntry struct {
	AffectedActivityID    string  `json:"affectedActivityId"`
	InterferingActivityID string  `json:"interferingActivityId"`
	PriorityWeight        float64 `json:"priorityWeight"`
}

type AlgorithmSelection struct {
	ID            string         `json:"id"`
	Configuration map[string]any `json:"configuration,omitempty"`
}

// PlanningSession owns one reproducible comparison of scheduling algorithms.
// Its inventory snapshot is frozen in Configuration by the application service.
type PlanningSession struct {
	ID                  string               `json:"id"`
	WorkflowVersionID   string               `json:"workflowVersionId"`
	ExecutionScopeID    string               `json:"executionScopeId"`
	NetworkTopologyID   string               `json:"networkTopologyId"`
	Status              PlanningStatus       `json:"status"`
	Algorithms          []AlgorithmSelection `json:"algorithms"`
	Progress            float64              `json:"progress"`
	CandidateCount      int                  `json:"candidateCount"`
	SelectedCandidateID string               `json:"selectedCandidateId,omitempty"`
	SelectedPlanID      string               `json:"selectedPlanId,omitempty"`
	DeadlineSeconds     float64              `json:"deadlineSeconds,omitempty"`
	Budget              float64              `json:"budget,omitempty"`
	Configuration       map[string]any       `json:"configuration,omitempty"`
	FailureReason       string               `json:"failureReason,omitempty"`
	CreatedAt           time.Time            `json:"createdAt"`
	StartedAt           *time.Time           `json:"startedAt,omitempty"`
	CompletedAt         *time.Time           `json:"completedAt,omitempty"`
}

type AlgorithmRun struct {
	ID                string           `json:"id"`
	PlanningSessionID string           `json:"planningSessionId"`
	Algorithm         string           `json:"algorithm"`
	Objective         string           `json:"objective"`
	Status            PlanningStatus   `json:"status"`
	Progress          float64          `json:"progress"`
	CandidateCount    int              `json:"candidateCount"`
	Configuration     map[string]any   `json:"configuration,omitempty"`
	Estimate          PlanningEstimate `json:"estimate"`
	FailureReason     string           `json:"failureReason,omitempty"`
	StartedAt         *time.Time       `json:"startedAt,omitempty"`
	CompletedAt       *time.Time       `json:"completedAt,omitempty"`
}

// PlanningEstimate describes the amount of search work expected before an
// algorithm starts. Clients combine it with live progress to estimate the
// remaining wall-clock time.
type PlanningEstimate struct {
	DurationSeconds     float64 `json:"durationSeconds"`
	ExpandedStates      int64   `json:"expandedStates"`
	ActivityCount       int     `json:"activityCount"`
	DependencyCount     int     `json:"dependencyCount"`
	CompatibleResources int     `json:"compatibleResources"`
	BeamWidth           int     `json:"beamWidth,omitempty"`
	ReadyBranchLimit    int     `json:"readyBranchLimit,omitempty"`
	Confidence          string  `json:"confidence"`
}

// PlanCandidate is intentionally separate from SchedulePlan. Only a selected
// candidate is promoted to the canonical plan aggregate used by execution.
type PlanCandidate struct {
	ID                string           `json:"id"`
	PlanningSessionID string           `json:"planningSessionId"`
	AlgorithmRunID    string           `json:"algorithmRunId"`
	Algorithm         string           `json:"algorithm"`
	Objective         string           `json:"objective"`
	Rank              int              `json:"rank"`
	ParetoOptimal     bool             `json:"paretoOptimal"`
	Dominated         bool             `json:"dominated"`
	Feasible          bool             `json:"feasible"`
	Predicted         PredictedMetrics `json:"predicted"`
	Plan              SchedulePlan     `json:"plan,omitempty"`
	Fingerprint       string           `json:"fingerprint"`
	CreatedAt         time.Time        `json:"createdAt"`
}
