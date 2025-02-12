package sdumont_runtime

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type SdumontRuntime struct {
}

func New() *SdumontRuntime {
	return &SdumontRuntime{}
}

func (s *SdumontRuntime) StartConnection() error {
	return nil
}

func (s *SdumontRuntime) StopConnection() error {
	return nil
}

func (s *SdumontRuntime) ApplyJob(workflowID int, activityID int) bool {
	return true
}

func (s *SdumontRuntime) DeleteJob(workflowID int, activityID int) bool {
	return true
}

func (s *SdumontRuntime) GetMetrics(workflowID int, activityID int) string {
	return ""
}

func (s *SdumontRuntime) GetLogs(workflow workflow_entity.Workflow, workflowActivity workflow_activity_entity.WorkflowActivities) string {
	return ""
}

func (s *SdumontRuntime) GetStatus(workflowID int, activityID int) string {
	return ""
}

func (s *SdumontRuntime) VerifyActivitiesWasFinished(workflow workflow_entity.Workflow) bool {
	return true
}

func NewSdumontRuntime() *SdumontRuntime {
	return &SdumontRuntime{}
}
