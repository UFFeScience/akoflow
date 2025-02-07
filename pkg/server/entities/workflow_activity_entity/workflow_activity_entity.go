package workflow_activity_entity

import (
	"encoding/base64"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type WorkflowActivities struct {
	Id           int
	WorkflowId   int
	Status       int
	Name         string   `yaml:"name"`
	Run          string   `yaml:"run"`
	Image        string   `yaml:"image"`
	MemoryLimit  string   `yaml:"memoryLimit"`
	CpuLimit     string   `yaml:"cpuLimit"`
	DependsOn    []string `yaml:"dependsOn"`
	NodeSelector string   `yaml:"nodeSelector"`
	KeepDisk     bool     `yaml:"keepDisk"`
	CreatedAt    string   `yaml:"createdAt"`
	StartedAt    string   `yaml:"startedAt"`
	FinishedAt   string   `yaml:"finishedAt"`
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
	CreatedAt         *string
	StartedAt         *string
	FinishedAt        *string
}

type WorkflowActivityDependencyDatabase struct {
	Id          int
	WorkflowId  int
	ActivityId  int
	DependsOnId int
}

type WorkflowPreActivityDatabase struct {
	Id                int
	ActivityId        int
	WorkflowId        int
	Namespace         string
	Name              string
	ResourceK8sBase64 *string
	Status            int
	Log               *string
}

func (wa WorkflowPreActivityDatabase) GetPreActivityName() string {
	return "preactivity-" + strconv.Itoa(wa.ActivityId)
}

func (wa WorkflowActivities) GetPreActivityName() string {
	return "preactivity-" + strconv.Itoa(wa.Id)
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

func (wfa WorkflowActivities) GetId() int {
	return wfa.Id
}

func (wfa WorkflowActivities) GetNodeSelector() map[string]string {
	wfaNodeSelector := wfa.NodeSelector

	if wfaNodeSelector == "" {
		return nil
	}

	split := strings.Split(wfaNodeSelector, "=")
	return map[string]string{split[0]: split[1]}
}

func (wfa WorkflowActivities) HasDependencies() bool {
	return len(wfa.DependsOn) > 0
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

	createdAt := ""
	if params.WorkflowActivityDatabase.CreatedAt != nil {
		createdAt = *params.WorkflowActivityDatabase.CreatedAt
	}

	startedAt := ""
	if params.WorkflowActivityDatabase.StartedAt != nil {
		startedAt = *params.WorkflowActivityDatabase.StartedAt
	}

	finishedAt := ""
	if params.WorkflowActivityDatabase.FinishedAt != nil {
		finishedAt = *params.WorkflowActivityDatabase.FinishedAt
	}

	return WorkflowActivities{
		Id:           params.WorkflowActivityDatabase.Id,
		Name:         params.WorkflowActivityDatabase.Name,
		Status:       params.WorkflowActivityDatabase.Status,
		Run:          wfa.Run,
		WorkflowId:   params.WorkflowActivityDatabase.WorkflowId,
		MemoryLimit:  wfa.MemoryLimit,
		CpuLimit:     wfa.CpuLimit,
		DependsOn:    wfa.DependsOn,
		NodeSelector: wfa.NodeSelector,
		KeepDisk:     wfa.KeepDisk,
		CreatedAt:    createdAt,
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
	}
}
