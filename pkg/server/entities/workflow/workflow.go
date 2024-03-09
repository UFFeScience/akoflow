package workflow

import (
	"encoding/base64"
	"gopkg.in/yaml.v3"
)

type Workflow struct {
	Name string       `yaml:"name"`
	Spec WorkflowSpec `yaml:"spec"`
	Id   int
}

type WorkflowSpec struct {
	MemoryLimit string               `yaml:"memoryLimit"`
	CPULimit    int                  `yaml:"cpuLimit"`
	Tries       int                  `yaml:"tries"`
	Image       string               `yaml:"image"`
	Activities  []WorkflowActivities `yaml:"activities"`
}

type WorkflowActivities struct {
	ID   int
	Name string `yaml:"name"`
	Run  string `yaml:"run"`
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
	ID                int
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
		ID:   params.WorkflowActivityDatabase.ID,
		Name: params.WorkflowActivityDatabase.Name,
		Run:  wfa.Run,
	}
}
