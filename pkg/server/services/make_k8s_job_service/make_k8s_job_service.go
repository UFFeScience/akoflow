package make_k8s_job_service

import (
	"encoding/base64"
	"errors"
	"math/rand"
	"strconv"

	"github.com/ovvesley/scik8sflow/pkg/server/entities/k8s_job_entity"
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/scik8sflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/scik8sflow/pkg/server/repository/workflow_repository"
)

type ParamsNewMakeK8sJobService struct {
	WorkflowRepository workflow_repository.IWorkflowRepository
	ActivityRepository activity_repository.IActivityRepository
}

// New creates a new MakeK8sJobService.
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

// MakeK8sJobService is a service that creates a k8s job that will be used to run an activity.
type MakeK8sJobService struct {
	namespace          string
	dependencies       []workflow_activity_entity.WorkflowActivities
	idWorkflow         int
	idWorkflowActivity int

	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository
}

// SetNamespace sets the namespace where the k8s job will be created.
func (m *MakeK8sJobService) SetNamespace(namespace string) *MakeK8sJobService {
	m.namespace = namespace
	return m
}

// SetDependencies sets the dependencies of the activity.
func (m *MakeK8sJobService) SetDependencies(dependencies []workflow_activity_entity.WorkflowActivities) *MakeK8sJobService {
	m.dependencies = dependencies
	return m
}

// SetIdWorkflow sets the id of the workflow that will be used to make the k8s job.
func (m *MakeK8sJobService) SetIdWorkflow(idWorkflow int) *MakeK8sJobService {
	m.idWorkflow = idWorkflow
	return m
}

// SetIdWorkflowActivity sets the id of the activity that will be used to make the k8s job.
func (m *MakeK8sJobService) SetIdWorkflowActivity(idWorkflowActivity int) *MakeK8sJobService {
	m.idWorkflowActivity = idWorkflowActivity
	return m
}

// getDependencies returns the dependencies of the activity.
func (m *MakeK8sJobService) getDependencies() []workflow_activity_entity.WorkflowActivities {
	return m.dependencies
}

// getIdWorkflow returns the id of the workflow that will be used to make the k8s job.
func (m *MakeK8sJobService) getIdWorkflow() int {
	return m.idWorkflow
}

// getIdWorkflowActivity returns the id of the activity that will be used to make the k8s job.
func (m *MakeK8sJobService) getIdWorkflowActivity() int {
	return m.idWorkflowActivity
}

// MakeK8sJob creates a k8s job that will be used to run the activity.
//
//	The k8s job will run the command that is defined in the activity.
//	The command is encoded in base64 and decoded in the container.
//
//	The k8s job will use a volume that is defined in the activity.
//	The volume is mounted in the container.
//
//	The k8s job will use the dependencies of the activity.
//	The dependencies are mounted in the container.
//
// - obs.1: the dependencies are the volumes that are defined in the dependencies of the activity.
// - obs.2: the dependencies are mounted in the container.
//
//	The k8s job will use the node selector that is defined in the activity.
//	The node selector is used to select the node that will run the activity.
//
// - obs.1: the node selector is defined in the activity.
//
//	The k8s job will use the resources that are defined in the activity.
func (m *MakeK8sJobService) MakeK8sJob() (k8s_job_entity.K8sJob, error) {
	if !m.isValidate() {
		return k8s_job_entity.K8sJob{}, errors.New("invalid parameters to make k8s job:: namespace, persistentVolumeClaim, dependencies, idWorkflow, idWorkflowActivity are required")
	}

	workflow, _ := m.workflowRepository.Find(m.getIdWorkflow())
	activity, _ := m.activityRepository.Find(m.getIdWorkflowActivity())

	container := m.makeContainer(workflow, activity)
	volumes := m.makeVolumes(workflow, activity)

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

	nodeSelector := m.makeNodeSelector(workflow, activity)
	if nodeSelector != nil {
		k8sJob.Spec.Template.Spec.NodeSelector = nodeSelector
	}

	return k8sJob, nil

}

// makeNodeSelector creates a node selector that will be used by the activity.
//   - The node selector is used to select the node that will run the activity.
//   - The node selector is defined in the activity.
func (m *MakeK8sJobService) makeNodeSelector(_ workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) map[string]string {
	nodeSelector := wfa.GetNodeSelector()
	return nodeSelector
}

// isValidate checks if the parameters are valid to make a k8s job.
//
//	The parameters are:
//	- namespace
//	- dependencies
//	- idWorkflow
//	- idWorkflowActivity
//
// The parameters should be set before calling the MakeK8sJob method.
func (m *MakeK8sJobService) isValidate() bool {
	return m.namespace != "" && m.dependencies != nil && m.idWorkflow != 0 && m.idWorkflowActivity != 0
}

// makeVolumes creates a list of volumes that will be used by the activity.
//
//	The first volume in the list is the volume that will be used by the current activity.
//	The other volumes are the dependencies of the current activity.
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

	firstVolume := m.makeVolumeThatWillBeUsedByCurrentActivity(wf, wfa)

	volumes = append([]k8s_job_entity.K8sJobVolume{firstVolume}, volumes...)

	return volumes
}

// makeVolumeThatWillBeUsedByCurrentActivity creates a volume that will be used by the current activity.
//
// This volume is the first volume in the list of volumes that will be used by the activity.
func (m *MakeK8sJobService) makeVolumeThatWillBeUsedByCurrentActivity(_ workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) k8s_job_entity.K8sJobVolume {
	firstVolume := k8s_job_entity.K8sJobVolume{
		Name: wfa.GetVolumeName(),
		PersistentVolumeClaim: struct {
			ClaimName string `json:"claimName"`
		}{
			ClaimName: wfa.GetVolumeName(),
		},
	}

	return firstVolume
}

// makeContainer creates a container that will be used by the activity.
//
//	The container will run the command that is defined in the activity.
//	  - obs.1: the command is encoded in base64.
//	  - obs.2: the command is decoded and executed in the container.
//	  - obs.3: kubernetes accept multiple containers in a pod, but we are using only one container to run activities.
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

// makeJobVolumeMountPath creates the path where the volume will be mounted in the container.
//   - The path is the mount path of the workflow concatenated with the name of the activity.
//   - The mount path of the workflow is defined in the workflow spec.
//   - The name of the activity is defined in the activity.
//
// the name of the activity should be lower case and without spaces, because it will be used as a directory name.
func (m *MakeK8sJobService) makeJobVolumeMountPath(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) string {
	return wf.Spec.MountPath + "/" + wfa.GetName()
}

// makeJobVolumeMounts creates a list of volume mounts that will be used by the container.
//
//   - The first volume mount in the list is the volume mount that will be used by the current activity.
//
//   - The other volume mounts are the dependencies of the current activity.
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
