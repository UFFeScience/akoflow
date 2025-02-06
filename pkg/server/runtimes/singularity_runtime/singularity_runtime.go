package singularity_runtime

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type SingularityRuntime struct {
}

func New() *SingularityRuntime {
	return &SingularityRuntime{}
}

func (s *SingularityRuntime) StartConnection() error {
	return nil
}

func (s *SingularityRuntime) StopConnection() error {
	return nil
}

func (s *SingularityRuntime) ApplyJob(workflowID int, activityID int) bool {
	return true
}

func (s *SingularityRuntime) DeleteJob(workflowID int, activityID int) bool {
	return true
}

func (s *SingularityRuntime) GetMetrics(workflowID int, activityID int) string {
	return ""
}

func (s *SingularityRuntime) GetLogs(workflow workflow_entity.Workflow, workflowActivity workflow_activity_entity.WorkflowActivities) string {
	return ""
}

func (s *SingularityRuntime) GetStatus(workflowID int, activityID int) string {
	return ""
}

func (s *SingularityRuntime) VerifyActivitiesWasFinished(workflow workflow_entity.Workflow) bool {
	return true
}

func NewSingularityRuntime() *SingularityRuntime {
	return &SingularityRuntime{}
}
