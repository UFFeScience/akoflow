package workflow_entity

import (
	"encoding/base64"
	"strconv"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"gopkg.in/yaml.v3"
)

type Workflow struct {
	Name string       `yaml:"name"`
	Spec WorkflowSpec `yaml:"spec"`
	Id   int
}

type WorkflowSpec struct {
	Image            string                                        `yaml:"image"`
	Namespace        string                                        `yaml:"namespace"`
	StorageClassName string                                        `yaml:"storageClassName"`
	StorageSize      string                                        `yaml:"storageSize"`
	StoragePolicy    WorkflowSpecStoragePolicy                     `yaml:"storagePolicy"`
	MountPath        string                                        `yaml:"mountPath"`
	Activities       []workflow_activity_entity.WorkflowActivities `yaml:"activities"`
}

type WorkflowSpecStoragePolicy struct {
	Type string `yaml:"type"` // "distributed" or "standalone"
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
	Activities     []workflow_activity_entity.WorkflowActivityDatabase
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

type ParamsDatabaseToWorkflow struct {
	WorkflowDatabase WorkflowDatabase
}

func DatabaseToWorkflow(params ParamsDatabaseToWorkflow) Workflow {
	return New(WorkflowNewParams{
		WorkflowBase64: params.WorkflowDatabase.RawWorkflow,
		Id:             &params.WorkflowDatabase.ID,
	})
}

func (w Workflow) GetVolumeName() string {
	return "pvc-" + strconv.Itoa(w.Id) + "-" + w.Name
}

func (w Workflow) IsStoragePolicyDistributed() bool {
	return w.Spec.StoragePolicy.Type == "distributed"
}

func (w Workflow) IsStoragePolicyStandalone() bool {
	return w.Spec.StoragePolicy.Type == "standalone" || w.Spec.StoragePolicy.Type == ""
}
