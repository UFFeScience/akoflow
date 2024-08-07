package make_k8s_job_service

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/k8s_job_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"math/rand"
	"strconv"
)

type MakeK8sActivityPreactivityService struct {
}

func newMakeK8sActivityPreactivityService() MakeK8sActivityPreactivityService {
	return MakeK8sActivityPreactivityService{}
}

func (m *MakeK8sActivityDistributedService) makeVolumesPreActivity(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) []k8s_job_entity.K8sJobVolume {
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

func (m *MakeK8sActivityDistributedService) makeContainerPreActivity(workflow workflow_entity.Workflow, activity workflow_activity_entity.WorkflowActivities) k8s_job_entity.K8sJobContainer {

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

// makeJobVolumeMountsPreactivity creates a list of volume mounts that will be used by the container.
//
//   - The first volume mount in the list is the volume mount that will be used by the current activity.
//
//   - The other volume mounts are the dependencies of the current activity.
func (m *MakeK8sActivityDistributedService) makeJobVolumeMountsPreactivity(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) []k8s_job_entity.K8sJobVolumeMount {
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

// makeVolumeThatWillBeUsedByCurrentActivity creates a volume that will be used by the current activity.
//
// This volume is the first volume in the list of volumes that will be used by the activity.
func (m *MakeK8sActivityDistributedService) makeVolumeThatWillBeUsedByCurrentActivity(_ workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) k8s_job_entity.K8sJobVolume {
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

// makeContainerActivity creates a container that will be used by the activity.
//
//	The container will run the command that is defined in the activity.
//	  - obs.1: the command is encoded in base64.
//	  - obs.2: the command is decoded and executed in the container.
//	  - obs.3: kubernetes accept multiple containers in a pod, but we are using only one container to run activities.
