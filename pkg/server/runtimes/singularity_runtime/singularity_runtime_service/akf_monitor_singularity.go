package singularity_runtime_service

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type AkfMonitorSingularity struct {
	FilePath         string
	Workflow         workflow_entity.Workflow
	WorkflowActivity workflow_activity_entity.WorkflowActivities
}

func NewAkfMonitorSingularity() *AkfMonitorSingularity {
	directory, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return &AkfMonitorSingularity{
		FilePath: filepath.Join(directory, "../../pkg/server/runtimes/singularity_runtime/singularity_runtime_service/akf_monitor_singularity.sh"),
	}
}

func (ams *AkfMonitorSingularity) ReadFile() ([]string, error) {
	file, err := os.Open(ams.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

func (ams *AkfMonitorSingularity) ReadFileAsString() (string, error) {
	lines, err := ams.ReadFile()
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

func (ams *AkfMonitorSingularity) SetWorkflow(workflow workflow_entity.Workflow) *AkfMonitorSingularity {
	ams.Workflow = workflow
	return ams
}

func (ams *AkfMonitorSingularity) SetWorkflowActivity(workflowActivity workflow_activity_entity.WorkflowActivities) *AkfMonitorSingularity {
	ams.WorkflowActivity = workflowActivity
	return ams
}

func (ams *AkfMonitorSingularity) GetWorkflow() workflow_entity.Workflow {
	return ams.Workflow
}

func (ams *AkfMonitorSingularity) GetWorkflowActivity() workflow_activity_entity.WorkflowActivities {
	return ams.WorkflowActivity
}

func (ams *AkfMonitorSingularity) GetScript() (string, error) {
	script, err := ams.ReadFileAsString()

	if err != nil {
		return "", err
	}

	if ams.GetWorkflowActivity().GetProcId() == "" {

		return "", nil
	}

	script = strings.ReplaceAll(script, "##PARENT_PID##", ams.GetWorkflowActivity().GetProcId())
	script = strings.ReplaceAll(script, "##WORKFLOW_ID##", strconv.Itoa(ams.GetWorkflow().GetId()))
	script = strings.ReplaceAll(script, "##WORKFLOW_ACTIVITY_ID##", strconv.Itoa(ams.GetWorkflowActivity().GetId()))
	script = strings.ReplaceAll(script, "##WORKFLOW_PATH_DATA_DIR##", ams.GetWorkflow().GetMountPath())

	if err != nil {
		return "", err
	}
	return script, nil
}
