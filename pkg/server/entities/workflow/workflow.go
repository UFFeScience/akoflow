package workflow

import (
	"encoding/base64"
	"github.com/ovvesley/scik8sflow/pkg/server/k8sjob"
	"gopkg.in/yaml.v3"
	"math/rand"
	"strconv"
)

type Workflow struct {
	Name string       `yaml:"name"`
	Spec WorkflowSpec `yaml:"spec"`
	Id   int
}

type WorkflowSpec struct {
	Image            string               `yaml:"image"`
	Namespace        string               `yaml:"namespace"`
	StorageClassName string               `yaml:"storageClassName"`
	StorageSize      string               `yaml:"storageSize"`
	MountPath        string               `yaml:"mountPath"`
	Activities       []WorkflowActivities `yaml:"activities"`
}

type WorkflowActivitiesDependsOn struct {
	Name string `yaml:"name"`
	Id   int    `yaml:"id"`
}

type WorkflowActivities struct {
	Id          int
	WorkflowId  int
	Status      int
	Name        string   `yaml:"name"`
	Run         string   `yaml:"run"`
	MemoryLimit string   `yaml:"memoryLimit"`
	CpuLimit    string   `yaml:"cpuLimit"`
	DependsOn   []string `yaml:"dependsOn"`
}

type WorkflowDatabase struct {
	ID          int
	Namespace   string
	Name        string
	RawWorkflow string
	Status      int
}

type WorkflowNewParams struct {
	WorkflowBase64 string
	Id             *int
	Activities     []WorkflowActivityDatabase
}

type WorkflowActivityDatabase struct {
	Id                int
	WorkflowId        int
	Namespace         string
	Name              string
	Image             string
	ResourceK8sBase64 string
	Status            int
	DependOnActivity  *int
}

func New(params WorkflowNewParams) Workflow {

	byteWorkflow, _ := base64.StdEncoding.DecodeString(params.WorkflowBase64)

	stringWorkflow := string(byteWorkflow)

	yamlWorkflow := Workflow{}
	err := yaml.Unmarshal([]byte(stringWorkflow), &yamlWorkflow)

	interfaceWorkflow := map[string]interface{}{}
	err = yaml.Unmarshal([]byte(stringWorkflow), &interfaceWorkflow)

	if params.Id != nil {
		yamlWorkflow.Id = *params.Id
	}

	if err != nil {
		return Workflow{}
	}

	return yamlWorkflow
}

func (w Workflow) ToYaml() string {
	return ""
}

func (w Workflow) Validate() bool {
	return true
}

func (w Workflow) GetBase64Workflow() string {
	y, _ := yaml.Marshal(w)
	return base64.StdEncoding.EncodeToString(y)
}

func (wa WorkflowActivities) GetBase64Activities() string {
	y, _ := yaml.Marshal(wa)
	return base64.StdEncoding.EncodeToString(y)
}

type ParamsDatabaseToWorkflow struct {
	WorkflowDatabase WorkflowDatabase
}

func DatabaseToWorkflow(params ParamsDatabaseToWorkflow) Workflow {
	return New(WorkflowNewParams{
		WorkflowBase64: params.WorkflowDatabase.RawWorkflow,
		Id:             &params.WorkflowDatabase.ID,
	})
}

type ParamsDatabaseToWorkflowActivities struct {
	WorkflowActivityDatabase WorkflowActivityDatabase
}

func DatabaseToWorkflowActivities(params ParamsDatabaseToWorkflowActivities) WorkflowActivities {

	activityDecoding, err := base64.StdEncoding.DecodeString(params.WorkflowActivityDatabase.ResourceK8sBase64)
	if err != nil {
		return WorkflowActivities{}
	}

	activityString := string(activityDecoding)

	wfa := WorkflowActivities{}
	err = yaml.Unmarshal([]byte(activityString), &wfa)
	if err != nil {
		return WorkflowActivities{}
	}

	return WorkflowActivities{
		Id:          params.WorkflowActivityDatabase.Id,
		Name:        params.WorkflowActivityDatabase.Name,
		Status:      params.WorkflowActivityDatabase.Status,
		Run:         wfa.Run,
		WorkflowId:  params.WorkflowActivityDatabase.WorkflowId,
		MemoryLimit: wfa.MemoryLimit,
		CpuLimit:    wfa.CpuLimit,
		DependsOn:   wfa.DependsOn,
	}
}

func (w Workflow) MakeResourcesK8s() []k8sjob.K8sJob {
	k8sjobs := make([]k8sjob.K8sJob, 0)
	for _, activity := range w.Spec.Activities {
		k8sJob := makeJobK8s(w, activity)
		k8sjobs = append(k8sjobs, k8sJob)
	}
	return k8sjobs
}

func (wa WorkflowActivities) MakeResourceK8s(workflow Workflow) k8sjob.K8sJob {
	return makeJobK8s(workflow, wa)
}

func makeJobK8s(workflow Workflow, activity WorkflowActivities) k8sjob.K8sJob {

	firstContainer := makeContainer(workflow, activity)
	firstVolume := makeVolume(workflow)

	k8sJob := k8sjob.K8sJob{
		ApiVersion: "batch/v1",
		Kind:       "Job",
		Metadata: k8sjob.K8sJobMetadata{
			//replace _ to - and add a random number
			Name: activity.GetName(),
		},
		Spec: k8sjob.K8sJobSpec{
			Template: k8sjob.K8sJobTemplate{
				Spec: k8sjob.K8sJobSpecTemplate{
					Containers:    []k8sjob.K8sJobContainer{firstContainer},
					RestartPolicy: "Never",
					Volumes:       []k8sjob.K8sJobVolume{firstVolume},
				},
			},
		},
	}

	return k8sJob
}

func makeVolume(workflow Workflow) k8sjob.K8sJobVolume {
	volume := k8sjob.K8sJobVolume{
		Name: workflow.GetVolumeName(),
		PersistentVolumeClaim: struct {
			ClaimName string `json:"claimName"`
		}{
			ClaimName: workflow.GetVolumeName(),
		},
	}
	return volume
}

func makeContainer(workflow Workflow, activity WorkflowActivities) k8sjob.K8sJobContainer {
	command := base64.StdEncoding.EncodeToString([]byte(activity.Run))

	container := k8sjob.K8sJobContainer{
		Name:    "activity-0" + strconv.Itoa(rand.Intn(100)),
		Image:   workflow.Spec.Image,
		Command: []string{"/bin/sh", "-c", "echo " + command + "| base64 -d| sh"},
		VolumeMounts: []k8sjob.K8sJobVolumeMount{
			{
				Name:      workflow.GetVolumeName(),
				MountPath: workflow.Spec.MountPath,
			},
		},
		Resources: k8sjob.K8sJobResources{
			Limits: k8sjob.K8sJobResourcesLimits{
				Cpu:    activity.CpuLimit,
				Memory: activity.MemoryLimit,
			},
		},
	}

	return container
}

func (wa WorkflowActivities) GetName() string {
	return "activity-" + strconv.Itoa(wa.Id)
}

func (w Workflow) GetVolumeName() string {
	return "pvc-" + strconv.Itoa(w.Id) + "-" + w.Name
}
