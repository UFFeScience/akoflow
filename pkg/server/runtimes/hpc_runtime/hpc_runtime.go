package hpc_runtime

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/runtimes/hpc_runtime/hpc_runtime_service"
)

func NewHpcRuntime() *HpcRuntime {
	return &HpcRuntime{
		hpcRuntimeService: hpc_runtime_service.New(),
	}
}

type HpcRuntime struct {
	hpcRuntimeService *hpc_runtime_service.HPCRuntimeService
	runtimeName       string
}

func (h *HpcRuntime) StartConnection() error {
	return nil
}

func (h *HpcRuntime) StopConnection() error {
	return nil
}

func (h *HpcRuntime) SetRuntimeName(runtimeName string) *HpcRuntime {
	h.runtimeName = runtimeName
	return h
}

func (h *HpcRuntime) ApplyJob(workflowID int, activityID int) bool {
	h.hpcRuntimeService.ApplyJob(workflowID, activityID)
	return true
}

func (h *HpcRuntime) DeleteJob(workflowID int, activityID int) bool {
	return true
}

func (h *HpcRuntime) GetMetrics(workflowID int, activityID int) string {
	return ""
}

func (h *HpcRuntime) GetLogs(workflow workflow_entity.Workflow, workflowActivity workflow_activity_entity.WorkflowActivities) string {
	return ""
}

func (h *HpcRuntime) GetStatus(workflowID int, activityID int) string {
	return ""
}

func (h *HpcRuntime) HealthCheck() bool {
	h.hpcRuntimeService.HealthCheck(h.runtimeName)
	return true
}

func (h *HpcRuntime) VerifyActivitiesWasFinished(workflow workflow_entity.Workflow) bool {
	h.hpcRuntimeService.VerifyActivitiesWasFinished(workflow)
	return true
}
