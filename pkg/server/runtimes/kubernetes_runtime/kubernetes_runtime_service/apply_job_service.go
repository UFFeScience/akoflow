package kubernetes_runtime_service

type ApplyJobService struct {
	applyJobStandaloneService  ApplyJobStandaloneService
	applyJobDistributedService ApplyJobDistributedService
}

func NewApplyJobService() ApplyJobService {
	return ApplyJobService{

		applyJobStandaloneService:  newApplyJobStandaloneService(),
		applyJobDistributedService: newApplyJobDistributedService(),
	}
}

func (a *ApplyJobService) ApplyJobStandalone(activityID int) {
	a.applyJobStandaloneService.ApplyStandaloneJob(activityID)
}

func (a *ApplyJobService) ApplyJobDistributed(activityID int) {
	a.applyJobDistributedService.ApplyDistributedJob(activityID)
}
