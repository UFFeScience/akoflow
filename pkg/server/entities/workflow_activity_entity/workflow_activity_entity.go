package workflow_activity_entity

import (
	"encoding/base64"
	"gopkg.in/yaml.v3"
	"strconv"
)

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

type WorkflowActivityDependencyDatabase struct {
	Id          int
	WorkflowId  int
	ActivityId  int
	DependsOnId int
}

type MapActivityDependencies map[int][]WorkflowActivities

func (wa WorkflowActivities) GetBase64Activities() string {
	y, _ := yaml.Marshal(wa)
	return base64.StdEncoding.EncodeToString(y)
}

func (wa WorkflowActivities) GetName() string {
	//return "activity-" + strconv.Itoa(wa.Id)
	return wa.Name
}

func (wa WorkflowActivities) GetNameJob() string {
	return "activity-" + strconv.Itoa(wa.Id) + "-" + wa.Name
}

func (wfa WorkflowActivities) GetVolumeName() string {
	return "pvc-" + strconv.Itoa(wfa.Id) + "-" + "wfa"
}

// get id
func (wfa WorkflowActivities) GetId() int {
	return wfa.Id
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

//
//func (wa WorkflowActivities) MakeResourceK8s(workflow workflow_entity.Workflow) k8sjob.K8sJob {
//	return makeJobK8s(workflow, wa)
//}