package local_runtime

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/runtimes/local_runtime/local_runtime_service"
)

type LocalRuntime struct {
	LocalRuntimeService local_runtime_service.LocalRuntimeService
	runtimeType         string
	runtimeName         string
}

func (d *LocalRuntime) SetRuntimeType(runtimeType string) *LocalRuntime {
	d.runtimeType = runtimeType
	return d
}

func (d *LocalRuntime) SetRuntimeName(name string) *LocalRuntime {
	d.runtimeName = name
	return d
}

func (d *LocalRuntime) StartConnection() error {
	return nil
}

func (d *LocalRuntime) StopConnection() error {
	return nil
}

func (d *LocalRuntime) ApplyJob(workflowID int, activityID int) bool {
	// apply job in local runtime
	d.LocalRuntimeService.ApplyJob(workflowID, activityID)

	return true
}

func (d *LocalRuntime) DeleteJob(workflowID int, activityID int) bool {
	return true
}

func (d *LocalRuntime) GetMetrics(workflowID int, activityID int) string {
	return ""
}

func (d *LocalRuntime) GetLogs(workflow workflow_entity.Workflow, workflowActivity workflow_activity_entity.WorkflowActivities) string {
	return ""
}

func (d *LocalRuntime) GetStatus(workflowID int, activityID int) string {
	return ""
}

func (k *LocalRuntime) VerifyActivitiesWasFinished(workflow workflow_entity.Workflow) bool {
	k.LocalRuntimeService.
		SetRuntimeName(k.runtimeName).
		SetRuntimeType(k.runtimeType).
		VerifyActivitiesWasFinished(workflow)
	return true
}

func (d *LocalRuntime) HealthCheck() bool {
	return true
}

func NewLocalRuntime() *LocalRuntime {
	return &LocalRuntime{
		LocalRuntimeService: local_runtime_service.NewLocalRuntimeService(),
	}
}
