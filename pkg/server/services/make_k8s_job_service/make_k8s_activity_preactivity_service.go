package make_k8s_job_service

import (
	"errors"
	"github.com/ovvesley/akoflow/pkg/server/entities/k8s_job_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"math/rand"
	"strconv"
)

type MakeK8sActivityPreactivityService struct {
	service *MakeK8sJobService
}

func newMakeK8sActivityPreactivityService() MakeK8sActivityPreactivityService {
	return MakeK8sActivityPreactivityService{
		service: nil,
	}
}

func (m *MakeK8sActivityPreactivityService) getDependencies() []workflow_activity_entity.WorkflowActivities {
	return m.service.GetDependencies()
}

func (m *MakeK8sActivityPreactivityService) isValidate() bool {
	return m.service.GetNamespace() != "" &&
		m.service.GetDependencies() != nil &&
		m.service.GetIdWorkflow() != 0 &&
		m.service.GetIdWorkflowActivity() != 0
}

func (m *MakeK8sActivityPreactivityService) Handle(service MakeK8sJobService) (k8s_job_entity.K8sJob, error) {
	m.setMakeK8sJobService(service)

	if !m.isValidate() {
		return k8s_job_entity.K8sJob{}, errors.New("invalid parameters to make k8s job:: namespace, persistentVolumeClaim, dependencies, idWorkflow, idWorkflowActivity are required")
	}

	workflow, _ := m.service.workflowRepository.Find(m.service.GetIdWorkflow())
	activity, _ := m.service.activityRepository.Find(m.service.GetIdWorkflowActivity())
	preActivity, _ := m.service.activityRepository.FindPreActivity(m.service.GetIdWorkflowActivity())

	volumes := m.makeVolumesPreActivity(workflow, activity)
	container := m.makeContainerPreActivity(workflow, activity)

	k8sJob := k8s_job_entity.K8sJob{
		ApiVersion: "batch/v1",
		Kind:       "Job",
		Metadata: k8s_job_entity.K8sJobMetadata{
			Name: activity.GetPreActivityName(),
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

	println("Running pre activity: ", preActivity.Name)
	println("Workflow: ", workflow.Name)
	println("Activity: ", activity.Name)

	return k8sJob, nil
}

func (m *MakeK8sActivityPreactivityService) setMakeK8sJobService(service MakeK8sJobService) *MakeK8sActivityPreactivityService {
	m.service = &service
	return m
}

func (m *MakeK8sActivityPreactivityService) makeVolumesPreActivity(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) []k8s_job_entity.K8sJobVolume {
	volumes := make([]k8s_job_entity.K8sJobVolume, 0)

	dependencies := m.getDependencies()

	for _, dependency := range dependencies {
		volume := k8s_job_entity.K8sJobVolume{
			Name: dependency.GetVolumeName(),
			PersistentVolumeClaim: struct {
				ClaimName string `json:"claimName"`
			}{
				ClaimName: dependency.GetVolumeName(),
			},
		}
		volumes = append(volumes, volume)
	}

	firstVolume := m.service.makeK8sActivityService.makeVolumeThatWillBeUsedByCurrentActivity(wf, wfa)

	volumes = append([]k8s_job_entity.K8sJobVolume{firstVolume}, volumes...)

	return volumes

}

func (m *MakeK8sActivityPreactivityService) makeContainerPreActivity(workflow workflow_entity.Workflow, activity workflow_activity_entity.WorkflowActivities) k8s_job_entity.K8sJobContainer {

	volumeMounts := m.makeJobVolumeMountsPreactivity(workflow, activity)

	firstVolumeMount := volumeMounts[0]

	container := k8s_job_entity.K8sJobContainer{
		Name:  "preactivity-0" + strconv.Itoa(rand.Intn(100)),
		Image: ImagePreActivity,
		Env: []k8s_job_entity.K8sJobEnv{
			{
				Name:  "WORKFLOW_NAME",
				Value: workflow.Name,
			},
			{
				Name:  "ACTIVITY_NAME",
				Value: activity.Name,
			},
			{
				Name:  "WORKFLOW_ID",
				Value: strconv.Itoa(workflow.Id),
			},
			{
				Name:  "ACTIVITY_ID",
				Value: strconv.Itoa(activity.Id),
			},
			{
				Name:  "OUTPUT_DIR",
				Value: firstVolumeMount.MountPath,
			},
			{
				Name:  "MOUNT_PATH",
				Value: workflow.Spec.MountPath,
			},
		},

		VolumeMounts: volumeMounts,
		Resources: k8s_job_entity.K8sJobResources{
			Limits: k8s_job_entity.K8sJobResourcesLimits{
				Cpu:    activity.CpuLimit,
				Memory: activity.MemoryLimit,
			},
		},
	}

	return container
}

func (m *MakeK8sActivityPreactivityService) makeJobVolumeMountsPreactivity(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) []k8s_job_entity.K8sJobVolumeMount {
	dependencies := m.getDependencies()

	volumesMounts := make([]k8s_job_entity.K8sJobVolumeMount, 0)

	for _, dependency := range dependencies {
		volumeMount := k8s_job_entity.K8sJobVolumeMount{
			Name:      dependency.GetVolumeName(),
			MountPath: m.service.makeK8sActivityService.makeJobVolumeMountPath(wf, dependency),
		}
		volumesMounts = append(volumesMounts, volumeMount)
	}

	firstVolumeMount := k8s_job_entity.K8sJobVolumeMount{
		Name:      wfa.GetVolumeName(),
		MountPath: m.service.makeK8sActivityService.makeJobVolumeMountPath(wf, wfa),
	}

	volumesMounts = append([]k8s_job_entity.K8sJobVolumeMount{firstVolumeMount}, volumesMounts...)

	return volumesMounts
}
