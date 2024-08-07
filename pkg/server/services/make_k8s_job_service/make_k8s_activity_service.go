package make_k8s_job_service

import (
	"encoding/base64"
	"github.com/ovvesley/akoflow/pkg/server/entities/k8s_job_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"math/rand"
	"os"
	"strconv"
)

type MakeK8sActivityService struct {
}

func newMakeK8sActivityService() MakeK8sActivityService {
	return MakeK8sActivityService{}
}

// makeContainerCommandActivity creates the command that will be used by the container.
//
//	The command is defined in the activity.
func (m *MakeK8sActivityService) makeContainerCommandActivity(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) string {

	command := m.setupCommandWorkdir(wf, wfa)

	command = m.addCommandToMonitorFilesStorage(command, "initial-file-list")
	command = m.addCommandToMonitorDiskSpecStorage(command, "initial-disk-spec")

	command += wfa.Run

	command = m.addCommandToMonitorFilesStorage(command, "end-file-list")
	command = m.addCommandToMonitorDiskSpecStorage(command, "end-disk-spec")

	return base64.StdEncoding.EncodeToString([]byte(command))

}

func (m *MakeK8sActivityService) setupCommandWorkdir(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) string {
	command := "mkdir -p " + wf.Spec.MountPath + "/" + wfa.GetName() + "; \n"
	command += "echo CURRENT_DIR: $(pwd); \n"
	command += "mv -fvu /akoflow-wfa-shared/* " + wf.Spec.MountPath + "/" + wfa.GetName() + "; \n"
	command += "cd " + wf.Spec.MountPath + "/" + wfa.GetName() + "; \n"

	command += "printenv; \n"
	return command
}
func (m *MakeK8sActivityService) getPortAkoFlowServer() string {
	port := os.Getenv("AKOFLOW_SERVER_SERVICE_SERVICE_PORT")
	if port == "" {
		port = "8080"
	}
	return port
}

func (m *MakeK8sActivityService) addCommandToMonitorFilesStorage(command string, path string) string {
	port := m.getPortAkoFlowServer()
	pathUrl := "http://$AKOFLOW_SERVER_SERVICE_SERVICE_HOST:" + port + "/akoflow-server/internal/storage/" + path + "?workflowId=" + strconv.Itoa(m.getIdWorkflow()) + "&activityId=" + strconv.Itoa(m.getIdWorkflowActivity())
	command += "\nwget --header=\"Content-Type: text/plain\" -O -  --post-data=\"$(ls -laR --full-time)\" \"" + pathUrl + "\" | true; \n"

	return command
}

func (m *MakeK8sActivityService) addCommandToMonitorDiskSpecStorage(command string, path string) string {
	port := m.getPortAkoFlowServer()
	pathUrl := "http://$AKOFLOW_SERVER_SERVICE_SERVICE_HOST:" + port + "/akoflow-server/internal/storage/" + path + "?workflowId=" + strconv.Itoa(m.getIdWorkflow()) + "&activityId=" + strconv.Itoa(m.getIdWorkflowActivity())
	command += "\nwget --header=\"Content-Type: text/plain\" -O - --post-data=\"$(df -h)\" \"" + pathUrl + "\" | true; \n"

	return command
}

// makeJobVolumeMountPath creates the path where the volume will be mounted in the container.
//   - The path is the mount path of the workflow concatenated with the name of the activity.
//   - The mount path of the workflow is defined in the workflow spec.
//   - The name of the activity is defined in the activity.
//
// the name of the activity should be lower case and without spaces, because it will be used as a directory name.
func (m *MakeK8sActivityService) makeJobVolumeMountPath(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) string {
	return wf.Spec.MountPath + "/" + wfa.GetName()
}

func (m *MakeK8sActivityService) makeJobVolumeMounts(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) []k8s_job_entity.K8sJobVolumeMount {

	volumesMounts := make([]k8s_job_entity.K8sJobVolumeMount, 0)

	firstVolumeMount := k8s_job_entity.K8sJobVolumeMount{
		Name:      wfa.GetVolumeName(),
		MountPath: m.makeJobVolumeMountPath(wf, wfa),
	}

	volumesMounts = append([]k8s_job_entity.K8sJobVolumeMount{firstVolumeMount}, volumesMounts...)

	return volumesMounts
}

func (m *MakeK8sJobService) makeContainerActivity(workflow workflow_entity.Workflow, activity workflow_activity_entity.WorkflowActivities) k8s_job_entity.K8sJobContainer {
	command := m.makeContainerCommandActivity(workflow, activity)

	envs := make([]k8s_job_entity.K8sJobEnv, 0)
	if os.Getenv("ENV") == "DEVELOPMENT" {
		envs = append(envs, k8s_job_entity.K8sJobEnv{
			Name:  "AKOFLOW_SERVER_SERVICE_SERVICE_HOST",
			Value: os.Getenv("AKOFLOW_SERVER_SERVICE_SERVICE_HOST"),
		})
	}

	envs = append(envs, k8s_job_entity.K8sJobEnv{
		Name:  "WORKFLOW_NAME",
		Value: workflow.Name,
	})

	envs = append(envs, k8s_job_entity.K8sJobEnv{
		Name:  "ACTIVITY_NAME",
		Value: activity.Name,
	})

	envs = append(envs, k8s_job_entity.K8sJobEnv{
		Name:  "WORKFLOW_ID",
		Value: strconv.Itoa(workflow.Id),
	})

	envs = append(envs, k8s_job_entity.K8sJobEnv{
		Name:  "ACTIVITY_ID",
		Value: strconv.Itoa(activity.Id),
	})

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
		Env: envs,
	}

	return container
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
// The parameters should be set before calling the MakeK8sActivityJob method.
func (m *MakeK8sJobService) isValidate() bool {
	return m.namespace != "" && m.dependencies != nil && m.idWorkflow != 0 && m.idWorkflowActivity != 0
}

// makeVolumesActivity creates a list of volumes that will be used by the activity.
//
//	The first volume in the list is the volume that will be used by the current activity.
//	The other volumes are the dependencies of the current activity.
func (m *MakeK8sJobService) makeVolumesActivity(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) []k8s_job_entity.K8sJobVolume {
	volumes := make([]k8s_job_entity.K8sJobVolume, 0)

	firstVolume := m.makeVolumeThatWillBeUsedByCurrentActivity(wf, wfa)

	volumes = append([]k8s_job_entity.K8sJobVolume{firstVolume}, volumes...)

	return volumes
}
