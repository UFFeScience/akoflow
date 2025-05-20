package sdumont_runtime_service

import (
	"encoding/base64"
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/entities/runtime_entity"
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
	runtime            runtime_entity.Runtime
}

func (m MakeSBatchSDumontActivityService) SetSingularityCommand(singularityCommand string) MakeSBatchSDumontActivityService {
	m.singularityCommand = singularityCommand
	return m
}

func (m MakeSBatchSDumontActivityService) SetRuntime(runtime runtime_entity.Runtime) MakeSBatchSDumontActivityService {
	m.runtime = runtime
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

	time := m.runtime.GetCurrentRuntimeMetadata("TIME")
	if time == "" {
		time = "48:00:00" // 48 hours
	}

	partition := m.runtime.GetCurrentRuntimeMetadata("QUEUE")
	if partition == "" {
		partition = "gdl"
	}

	ntasksStr := m.runtime.GetCurrentRuntimeMetadata("NTASKS")
	ntasks := 1
	if ntasksStr != "" {
		fmt.Sscanf(ntasksStr, "%d", &ntasks)
	}

	nodesStr := m.runtime.GetCurrentRuntimeMetadata("NODES")
	nodes := 1
	if nodesStr != "" {
		fmt.Sscanf(nodesStr, "%d", &nodes)
	}

	gpusStr := m.runtime.GetCurrentRuntimeMetadata("GPUS")
	gpus := 1
	if gpusStr != "" {
		fmt.Sscanf(gpusStr, "%d", &gpus)
	}

	cpusPerGpuStr := m.runtime.GetCurrentRuntimeMetadata("CPUS_PER_GPU")
	cpusPerGpu := 1
	if cpusPerGpuStr != "" {
		fmt.Sscanf(cpusPerGpuStr, "%d", &cpusPerGpu)
	}

	mem := m.runtime.GetCurrentRuntimeMetadata("MEM")
	if mem == "" {
		mem = "8G"
	}

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
