package kubernetes_runtime_service

import (
	"errors"

	"github.com/ovvesley/akoflow/pkg/server/entities/k8s_job_entity"
)

type MakeK8sActivityStandaloneService struct {
	service *MakeK8sJobService
}

func newMakeK8sActivityStandaloneService() MakeK8sActivityStandaloneService {
	return MakeK8sActivityStandaloneService{}
}
func (m *MakeK8sActivityStandaloneService) isValidate() bool {
	return m.service.GetNamespace() != "" &&
		m.service.GetDependencies() != nil &&
		m.service.GetIdWorkflow() != 0 &&
		m.service.GetIdWorkflowActivity() != 0
}

func (m *MakeK8sActivityStandaloneService) Handle(service MakeK8sJobService) (k8s_job_entity.K8sJob, error) {
	m.SetMakeK8sJobService(service)

	if !m.isValidate() {
		return k8s_job_entity.K8sJob{}, errors.New("invalid parameters to make k8s job:: namespace, persistentVolumeClaim, dependencies, idWorkflow, idWorkflowActivity are required")
	}

	workflow, _ := m.service.workflowRepository.Find(m.service.GetIdWorkflow())
	activity, _ := m.service.activityRepository.Find(m.service.GetIdWorkflowActivity())

	container := m.service.makeK8sActivityService.makeContainerActivity(workflow, activity)
	volumes := m.service.makeK8sActivityService.makeVolumesActivity(workflow, activity)

	k8sJob := k8s_job_entity.K8sJob{
		ApiVersion: "batch/v1",
		Kind:       "Job",
		Metadata: k8s_job_entity.K8sJobMetadata{
			Name: activity.GetNameJob(),
		},
		Spec: k8s_job_entity.K8sJobSpec{
			BackoffLimit: 0,
			Template: k8s_job_entity.K8sJobTemplate{
				Spec: k8s_job_entity.K8sJobSpecTemplate{
					Containers:    []k8s_job_entity.K8sJobContainer{container},
					RestartPolicy: "Never",
					Volumes:       volumes,
				},
			},
		},
	}

	nodeSelector := m.service.makeK8sActivityService.MakeNodeSelector(workflow, activity)
	if nodeSelector != nil {
		k8sJob.Spec.Template.Spec.NodeSelector = nodeSelector
	}

	return k8sJob, nil

}

func (m *MakeK8sActivityStandaloneService) SetMakeK8sJobService(service MakeK8sJobService) *MakeK8sActivityStandaloneService {
	m.service = &service
	return m
}
