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
	Workflow           workflow_entity.Workflow
	IdWorkflowActivity int
}

func newMakeK8sActivityService() MakeK8sActivityService {
	return MakeK8sActivityService{}
}

func (m *MakeK8sActivityService) SetIdWorkflowActivity(idWorkflowActivity int) *MakeK8sActivityService {
	m.IdWorkflowActivity = idWorkflowActivity
	return m
}

func (m *MakeK8sActivityService) getIdWorkflowActivity() int {
	return m.IdWorkflowActivity
}

func (m *MakeK8sActivityService) SetWorkflow(workflow workflow_entity.Workflow) *MakeK8sActivityService {
	m.Workflow = workflow
	return m
}

func (m *MakeK8sActivityService) getIdWorkflow() int {
	return m.Workflow.Id
}

func (m *MakeK8sActivityService) GetWorkflow() workflow_entity.Workflow {
	return m.Workflow
}

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

	workdir := wf.Spec.MountPath + "/" + wfa.GetName()

	if wf.IsStoragePolicyDistributed() {
		workdir = wf.Spec.MountPath
	}

	command := "mkdir -p " + workdir + "; \n"
	command += "echo CURRENT_DIR: $(pwd); \n"
	command += "mv -fvu /akoflow-wfa-shared/* " + workdir + "; \n"
	command += "cd " + workdir + "; \n"

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

	if wf.IsStoragePolicyDistributed() {
		return wf.Spec.MountPath
	}

	return wf.Spec.MountPath + "/" + wfa.GetName()
}

func (m *MakeK8sActivityService) makeJobVolumeMounts(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) []k8s_job_entity.K8sJobVolumeMount {

	volumesMounts := make([]k8s_job_entity.K8sJobVolumeMount, 0)

	volumeName := wfa.GetVolumeName()

	if wf.IsStoragePolicyDistributed() {
		volumeName = wf.MakeWorkflowPersistentVolumeClaimName()
	}

	firstVolumeMount := k8s_job_entity.K8sJobVolumeMount{
		Name:      volumeName,
		MountPath: m.makeJobVolumeMountPath(wf, wfa),
	}

	volumesMounts = append([]k8s_job_entity.K8sJobVolumeMount{firstVolumeMount}, volumesMounts...)

	return volumesMounts
}

func (m *MakeK8sActivityService) makeContainerActivity(workflow workflow_entity.Workflow, activity workflow_activity_entity.WorkflowActivities) k8s_job_entity.K8sJobContainer {
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

// MakeNodeSelector creates a node selector that will be used by the activity.
//   - The node selector is used to select the node that will run the activity.
//   - The node selector is defined in the activity.
func (m *MakeK8sActivityService) MakeNodeSelector(_ workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) map[string]string {
	nodeSelector := wfa.GetNodeSelector()
	return nodeSelector
}

// makeVolumesActivity creates a list of volumes that will be used by the activity.
//
//	The first volume in the list is the volume that will be used by the current activity.
//	The other volumes are the dependencies of the current activity.
func (m *MakeK8sActivityService) makeVolumesActivity(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) []k8s_job_entity.K8sJobVolume {
	volumes := make([]k8s_job_entity.K8sJobVolume, 0)

	firstVolume := m.makeVolumeThatWillBeUsedByCurrentActivity(wf, wfa)

	volumes = append([]k8s_job_entity.K8sJobVolume{firstVolume}, volumes...)

	return volumes
}

func (m *MakeK8sActivityService) makeVolumeThatWillBeUsedByCurrentActivity(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) k8s_job_entity.K8sJobVolume {
	volumeName := wfa.GetVolumeName()

	if wf.IsStoragePolicyDistributed() {
		volumeName = wf.MakeWorkflowPersistentVolumeClaimName()
	}

	firstVolume := k8s_job_entity.K8sJobVolume{
		Name: volumeName,
		PersistentVolumeClaim: struct {
			ClaimName string `json:"claimName"`
		}{
			ClaimName: volumeName,
		},
	}

	return firstVolume
}