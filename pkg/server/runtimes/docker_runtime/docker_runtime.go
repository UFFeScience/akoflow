package docker_runtime

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type DockerRuntime struct {
}

func New() *DockerRuntime {
	return &DockerRuntime{}
}

func (d *DockerRuntime) StartConnection() error {
	return nil
}

func (d *DockerRuntime) StopConnection() error {
	return nil
}

func (d *DockerRuntime) ApplyJob(workflowID int, activityID int) bool {
	return true
}

func (d *DockerRuntime) DeleteJob(workflowID int, activityID int) bool {
	return true
}

func (d *DockerRuntime) GetMetrics(workflowID int, activityID int) string {
	return ""
}

func (d *DockerRuntime) GetLogs(workflow workflow_entity.Workflow, workflowActivity workflow_activity_entity.WorkflowActivities) string {
	return ""
}

func (d *DockerRuntime) GetStatus(workflowID int, activityID int) string {
	return ""
}

func (k *DockerRuntime) VerifyActivitiesWasFinished(workflow workflow_entity.Workflow) bool {
	return true
}

func (d *DockerRuntime) HealthCheck() bool {
	return true
}

func NewDockerRuntime() *DockerRuntime {
	return &DockerRuntime{}
}
