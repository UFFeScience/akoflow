package docker_runtime

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

func (d *DockerRuntime) GetLogs(workflowID int, activityID int) string {
	return ""
}

func (d *DockerRuntime) GetStatus(workflowID int, activityID int) string {
	return ""
}

func NewDockerRuntime() *DockerRuntime {
	return &DockerRuntime{}
}
