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
	ProcId       string   `yaml:"procId"`
	Name         string   `yaml:"name"`
	Run          string   `yaml:"run"`
	Image        string   `yaml:"image"`
	Runtime      string   `yaml:"runtime"`
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
	Runtime           string
	ResourceK8sBase64 string
	Status            int
	ProcId            *string
	DependOnActivity  *int
	NodeSelector      *string
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

func (wfa WorkflowActivities) GetProcId() string {
	return wfa.ProcId
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

	procId := ""
	if params.WorkflowActivityDatabase.ProcId != nil {
		procId = *params.WorkflowActivityDatabase.ProcId
	}

	runtime := ""
	if params.WorkflowActivityDatabase.Runtime != "" {
		runtime = params.WorkflowActivityDatabase.Runtime
	} else {
		runtime = wfa.Runtime
	}

	return WorkflowActivities{
		Id:           params.WorkflowActivityDatabase.Id,
		Name:         params.WorkflowActivityDatabase.Name,
		Status:       params.WorkflowActivityDatabase.Status,
		ProcId:       procId,
		Run:          wfa.Run,
		Image:        params.WorkflowActivityDatabase.Image,
		Runtime:      runtime,
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

func (wfa WorkflowActivities) GetMemoryRequired() float64 {
	if wfa.MemoryLimit == "" {
		return 0.0
	}

	memoryRequired, err := strconv.ParseFloat(strings.TrimSuffix(wfa.MemoryLimit, "Mi"), 64)
	if err != nil {
		panic("Error parsing memory limit: " + err.Error())
	}

	return memoryRequired
}

func (wfa WorkflowActivities) GetCpuRequired() float64 {
	if wfa.CpuLimit == "" {
		return 0.0
	}

	cpuRequired, err := strconv.ParseFloat(strings.TrimSuffix(wfa.CpuLimit, "m"), 64)
	if err != nil {
		panic("Error parsing CPU limit: " + err.Error())
	}

	return cpuRequired
}

func (wfa WorkflowActivities) GetRuntimeId() string {
	if wfa.Runtime != "" {
		return wfa.Runtime
	}

	panic("Runtime not set")
}

func (wfa WorkflowActivities) HasNodeSelector() bool {
	return wfa.NodeSelector != ""
}
