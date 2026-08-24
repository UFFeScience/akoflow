package planning

import (
	"github.com/UFFeScience/akoflow/internal/domain/environment"
	"github.com/UFFeScience/akoflow/internal/domain/resource"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
)

type PlanningSource string
type ExecutionMode string

const (
	PlanningSourcePlugin     PlanningSource = "plugin"
	PlanningSourceImported   PlanningSource = "imported"
	ExecutionModeReal        ExecutionMode  = "real"
	ExecutionModeSimulation  ExecutionMode  = "simulation"
	ExecutionModeInteractive ExecutionMode  = "interactive"
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
