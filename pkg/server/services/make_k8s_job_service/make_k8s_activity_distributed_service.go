package make_k8s_job_service

import "github.com/ovvesley/akoflow/pkg/server/entities/k8s_job_entity"

type MakeK8sActivityDistributedService struct {
}

func newMakeK8sActivityDistributedService() MakeK8sActivityDistributedService {
	return MakeK8sActivityDistributedService{}
}

func (m *MakeK8sActivityDistributedService) Handle(service MakeK8sJobService) (k8s_job_entity.K8sJob, error) {
	return k8s_job_entity.K8sJob{}, nil
}
