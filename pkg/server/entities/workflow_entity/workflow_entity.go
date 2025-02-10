package workflow_entity

import (
	"encoding/base64"
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"gopkg.in/yaml.v3"
)

type Workflow struct {
	Name   string       `yaml:"name"`
	Spec   WorkflowSpec `yaml:"spec"`
	Id     int
	Status int `yaml:"status"`
}

const MODE_DISTRIBUTED = "distributed"
const MODE_STANDALONE = "standalone"

type WorkflowSpec struct {
	Runtime       string                                        `yaml:"runtime"`
	Image         string                                        `yaml:"image"`
	StoragePolicy WorkflowSpecStoragePolicy                     `yaml:"storagePolicy"`
	MountPath     string                                        `yaml:"mountPath"`
	Activities    []workflow_activity_entity.WorkflowActivities `yaml:"activities"`
	Namespace     string                                        `yaml:"namespace"`
}

type WorkflowSpecStoragePolicy struct {
	Type             string `yaml:"type"` // "distributed", "standalone" or "default"
	StorageClassName string `yaml:"storageClassName"`
	StorageSize      string `yaml:"storageSize"`
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
	Status         *int
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

	if params.Status != nil {
		yamlWorkflow.Status = *params.Status
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
		Status:         &params.WorkflowDatabase.Status,
	})
}

func (w Workflow) IsStoragePolicyDistributed() bool {
	return w.Spec.StoragePolicy.Type == MODE_DISTRIBUTED
}

func (w Workflow) IsStoragePolicyStandalone() bool {
	return w.Spec.StoragePolicy.Type == MODE_STANDALONE || w.Spec.StoragePolicy.Type == ""
}

func (w Workflow) GetMode() string {
	if w.IsStoragePolicyDistributed() {
		return MODE_DISTRIBUTED
	}

	if w.IsStoragePolicyStandalone() {
		return MODE_STANDALONE
	}

	return ""
}

func (w Workflow) GetId() int {
	return w.Id
}

func (w Workflow) MakeVolumeNameDistributed() string {
	return "wf-volume-" + fmt.Sprintf("%d", w.Id)
}

func (w Workflow) GetNamespace() string {
	return w.Spec.Namespace
}

func (w Workflow) GetStorageClassName() string {
	return w.Spec.StoragePolicy.StorageClassName
}

func (w Workflow) GetStorageSize() string {
	return w.Spec.StoragePolicy.StorageSize
}

func (w Workflow) GetStoragePolicyType() string {
	return w.Spec.StoragePolicy.Type
}

func (w Workflow) GetMountPath() string {
	return w.Spec.MountPath
}

func (w Workflow) GetRuntimeId() string {
	return w.Spec.Runtime
}

func (w Workflow) MakeStorageClassNameDistributed() string {
	return "akoflow-nfs-" + fmt.Sprintf("%d", w.Id)
}

func (w Workflow) MakeWorkflowPersistentVolumeClaimName() string {
	return "wf-pvc-" + fmt.Sprintf("%d", w.Id) + "-nfs"
}
