package sdumont_runtime_service

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

func NewMakeSBatchSDumontActivityService() MakeSBatchSDumontActivityService {
	return MakeSBatchSDumontActivityService{
		singularityCommand: "",
	}
}

type MakeSBatchSDumontActivityService struct {
	singularityCommand string
}

func (m MakeSBatchSDumontActivityService) SetSingularityCommand(singularityCommand string) MakeSBatchSDumontActivityService {
	m.singularityCommand = singularityCommand
	return m
}

func (m MakeSBatchSDumontActivityService) GetSingularityCommand() string {
	return m.singularityCommand
}

func (m MakeSBatchSDumontActivityService) Handle(workflow workflow_entity.Workflow, activity workflow_activity_entity.WorkflowActivities) string {

	if m.GetSingularityCommand() == "" {
		fmt.Println("Singularity command is empty")
		return ""
	}

	jobName := fmt.Sprintf("akoflow_%d_%d", workflow.GetId(), activity.GetId())

	output := fmt.Sprintf("%s/akoflow_out_%d_%d.out",
		workflow.GetMountPath(),
		workflow.GetId(),
		activity.GetId(),
	)

	error := fmt.Sprintf("%s/akoflow_err_%d_%d.err",
		workflow.GetMountPath(),
		workflow.GetId(),
		activity.GetId(),
	)

	time := "02:00:00"
	partition := os.Getenv("SDUMONT_QUEUE")
	ntasks := 1
	nodes := 1
	gpus := 1
	cpusPerGpu := 1
	mem := "8G"

	wrap := fmt.Sprintf("%s", m.GetSingularityCommand())

	command := fmt.Sprintf("sbatch --job-name=%s --output=%s --error=%s --time=%s --partition=%s --ntasks=%d --nodes=%d --gpus=%d --cpus-per-gpu=%d --mem=%s --wrap=\"%s\"",
		jobName,
		output,
		error,
		time,
		partition,
		ntasks,
		nodes,
		gpus,
		cpusPerGpu,
		mem,
		wrap,
	)

	base64ParcialCommand := base64.StdEncoding.EncodeToString([]byte(command))

	commandBase64 := fmt.Sprintf("echo %s | base64 -d | bash", base64ParcialCommand)

	return commandBase64
}
