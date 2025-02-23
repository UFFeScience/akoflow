package sdumont_runtime

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/runtimes/sdumont_runtime/sdumont_runtime_service.go"
)

func NewSdumontRuntime() *SdumontRuntime {
	return &SdumontRuntime{
		sDumontRuntimeService: sdumont_runtime_service.New(),
	}
}

type SdumontRuntime struct {
	sDumontRuntimeService *sdumont_runtime_service.SDumontRuntimeService
}

func (s *SdumontRuntime) StartConnection() error {
	return nil
}

func (s *SdumontRuntime) StopConnection() error {
	return nil
}

func (s *SdumontRuntime) ApplyJob(workflowID int, activityID int) bool {
	s.sDumontRuntimeService.ApplyJob(workflowID, activityID)
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
