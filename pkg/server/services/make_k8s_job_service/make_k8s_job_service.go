package make_k8s_job_service

import (
	"encoding/base64"
	"errors"
	"github.com/ovvesley/scik8sflow/pkg/server/entities/k8s_job_entity"
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/scik8sflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/scik8sflow/pkg/server/repository/workflow_repository"
	"math/rand"
	"strconv"
)

type ParamsNewMakeK8sJobService struct {
	WorkflowRepository workflow_repository.IWorkflowRepository
	ActivityRepository activity_repository.IActivityRepository
}

func New(params ...ParamsNewMakeK8sJobService) MakeK8sJobService {
	if len(params) > 0 {
		return MakeK8sJobService{
			workflowRepository: params[0].WorkflowRepository,
			activityRepository: params[0].ActivityRepository,
		}
	}
	return MakeK8sJobService{
		workflowRepository: workflow_repository.New(),
		activityRepository: activity_repository.New(),
	}

}

type MakeK8sJobService struct {
	namespace          string
	dependencies       []workflow_activity_entity.WorkflowActivities
	idWorkflow         int
	idWorkflowActivity int

	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository
}

func (m *MakeK8sJobService) SetNamespace(namespace string) *MakeK8sJobService {
	m.namespace = namespace
	return m
}

func (m *MakeK8sJobService) SetDependencies(dependencies []workflow_activity_entity.WorkflowActivities) *MakeK8sJobService {
	m.dependencies = dependencies
	return m
}

func (m *MakeK8sJobService) SetIdWorkflow(idWorkflow int) *MakeK8sJobService {
	m.idWorkflow = idWorkflow
	return m
}

func (m *MakeK8sJobService) SetIdWorkflowActivity(idWorkflowActivity int) *MakeK8sJobService {
	m.idWorkflowActivity = idWorkflowActivity
	return m
}

func (m *MakeK8sJobService) getDependencies() []workflow_activity_entity.WorkflowActivities {
	return m.dependencies
}

func (m *MakeK8sJobService) getIdWorkflow() int {
	return m.idWorkflow
}

func (m *MakeK8sJobService) getIdWorkflowActivity() int {
	return m.idWorkflowActivity
}

func (m *MakeK8sJobService) MakeK8sJob() (k8s_job_entity.K8sJob, error) {
	if !m.isValidate() {
		return k8s_job_entity.K8sJob{}, errors.New("invalid parameters to make k8s job:: namespace, persistentVolumeClaim, dependencies, idWorkflow, idWorkflowActivity are required")
	}

	workflow, _ := m.workflowRepository.Find(m.getIdWorkflow())
	activity, _ := m.activityRepository.Find(m.getIdWorkflowActivity())

	container := m.makeContainer(workflow, activity)
	volumes := m.makeVolumes(workflow, activity)

	println("container", string(container.Name))
	println("volumes", string(volumes[0].Name))

	k8sJob := k8s_job_entity.K8sJob{
		ApiVersion: "batch/v1",
		Kind:       "Job",
		Metadata: k8s_job_entity.K8sJobMetadata{
			Name: activity.GetNameJob(),
		},
		Spec: k8s_job_entity.K8sJobSpec{
			Template: k8s_job_entity.K8sJobTemplate{
				Spec: k8s_job_entity.K8sJobSpecTemplate{
					Containers:    []k8s_job_entity.K8sJobContainer{container},
					RestartPolicy: "Never",
					Volumes:       volumes,
				},
			},
		},
	}

	return k8sJob, nil

}

func (m *MakeK8sJobService) isValidate() bool {
	return m.namespace != "" && m.dependencies != nil && m.idWorkflow != 0 && m.idWorkflowActivity != 0
}

func (m *MakeK8sJobService) makeVolumes(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) []k8s_job_entity.K8sJobVolume {
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

	firstVolume := k8s_job_entity.K8sJobVolume{
		Name: wfa.GetVolumeName(),
		PersistentVolumeClaim: struct {
			ClaimName string `json:"claimName"`
		}{
			ClaimName: wfa.GetVolumeName(),
		},
	}

	volumes = append([]k8s_job_entity.K8sJobVolume{firstVolume}, volumes...)

	return volumes
}

func (m *MakeK8sJobService) makeContainer(workflow workflow_entity.Workflow, activity workflow_activity_entity.WorkflowActivities) k8s_job_entity.K8sJobContainer {
	command := base64.StdEncoding.EncodeToString([]byte(activity.Run))

	container := k8s_job_entity.K8sJobContainer{
		Name:         "activity-0" + strconv.Itoa(rand.Intn(100)),
		Image:        workflow.Spec.Image,
		Command:      []string{"/bin/sh", "-c", "echo " + command + "| base64 -d| sh"},
		VolumeMounts: m.makeJobVolumeMounts(workflow, activity),
		Resources: k8s_job_entity.K8sJobResources{
			Limits: k8s_job_entity.K8sJobResourcesLimits{
				Cpu:    activity.CpuLimit,
				Memory: activity.MemoryLimit,
			},
		},
	}

	return container
}

func (m *MakeK8sJobService) makeJobVolumeMountPath(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) string {
	return wf.Spec.MountPath + "/" + wfa.GetName()
}

func (m *MakeK8sJobService) makeJobVolumeMounts(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) []k8s_job_entity.K8sJobVolumeMount {
	dependencies := m.getDependencies()

	volumesMounts := make([]k8s_job_entity.K8sJobVolumeMount, 0)

	for _, dependency := range dependencies {
		volumeMount := k8s_job_entity.K8sJobVolumeMount{
			Name:      dependency.GetVolumeName(),
			MountPath: m.makeJobVolumeMountPath(wf, dependency),
		}
		volumesMounts = append(volumesMounts, volumeMount)
	}

	firstVolumeMount := k8s_job_entity.K8sJobVolumeMount{
		Name:      wfa.GetVolumeName(),
		MountPath: m.makeJobVolumeMountPath(wf, wfa),
	}

	volumesMounts = append([]k8s_job_entity.K8sJobVolumeMount{firstVolumeMount}, volumesMounts...)

	return volumesMounts
}
