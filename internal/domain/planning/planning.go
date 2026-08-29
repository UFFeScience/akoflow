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

type PlanningRequest struct {
	Workflow         workflow.WorkflowVersion           `json:"workflow"`
	ExecutionScope   environment.ExecutionScope         `json:"executionScope"`
	Environments     []environment.EnvironmentVersion   `json:"environments"`
	Resources        []resource.Resource                `json:"resources"`
	NetworkTopology  resource.NetworkTopology           `json:"networkTopology"`
	ActivityProfiles []workflow.ActivityResourceProfile `json:"activityProfiles"`
	DeadlineSeconds  float64                            `json:"deadlineSeconds"`
	Budget           float64                            `json:"budget"`
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
	ID                string         `json:"id"`
	PlanningSessionID string         `json:"planningSessionId"`
	Algorithm         string         `json:"algorithm"`
	Objective         string         `json:"objective"`
	Status            PlanningStatus `json:"status"`
	Progress          float64        `json:"progress"`
	CandidateCount    int            `json:"candidateCount"`
	Configuration     map[string]any `json:"configuration,omitempty"`
	FailureReason     string         `json:"failureReason,omitempty"`
	StartedAt         *time.Time     `json:"startedAt,omitempty"`
	CompletedAt       *time.Time     `json:"completedAt,omitempty"`
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
