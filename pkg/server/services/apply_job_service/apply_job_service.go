package apply_job_service

type ApplyJobService struct {
	applyJobStandaloneService  ApplyJobStandaloneService
	applyJobDistributedService ApplyJobDistributedService
}

func New() ApplyJobService {
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
