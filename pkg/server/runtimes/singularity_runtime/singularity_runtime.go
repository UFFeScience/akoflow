package singularity_runtime

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

func (s *SingularityRuntime) GetLogs(workflowID int, activityID int) string {
	return ""
}

func (s *SingularityRuntime) GetStatus(workflowID int, activityID int) string {
	return ""
}

func NewSingularityRuntime() *SingularityRuntime {
	return &SingularityRuntime{}
}
