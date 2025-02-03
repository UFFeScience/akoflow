package sdumont_runtime

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

func (s *SdumontRuntime) GetLogs(workflowID int, activityID int) string {
	return ""
}

func (s *SdumontRuntime) GetStatus(workflowID int, activityID int) string {
	return ""
}

func NewSdumontRuntime() *SdumontRuntime {
	return &SdumontRuntime{}
}
