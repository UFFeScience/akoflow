package hpc_runtime_service

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/entities/runtime_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

func NewMakeSBatchHPCRuntimeActivityService() MakeSBatchHPCRuntimeActivityService {
	return MakeSBatchHPCRuntimeActivityService{
		singularityCommand: "",
	}
}

type MakeSBatchHPCRuntimeActivityService struct {
	singularityCommand string
	runtime            runtime_entity.Runtime
}

func (m MakeSBatchHPCRuntimeActivityService) SetSingularityCommand(singularityCommand string) MakeSBatchHPCRuntimeActivityService {
	m.singularityCommand = singularityCommand
	return m
}

func (m MakeSBatchHPCRuntimeActivityService) SetRuntime(runtime runtime_entity.Runtime) MakeSBatchHPCRuntimeActivityService {
	m.runtime = runtime
	return m
}

func (m MakeSBatchHPCRuntimeActivityService) GetSingularityCommand() string {
	return m.singularityCommand
}

func (m MakeSBatchHPCRuntimeActivityService) Handle(workflow workflow_entity.Workflow, activity workflow_activity_entity.WorkflowActivities) string {

	templateSbatchb64 := m.runtime.GetCurrentRuntimeMetadata("SBATCHTEMPLATE")
	templateSbatchBytes, err := base64.StdEncoding.DecodeString(templateSbatchb64)
	if err != nil {
		fmt.Println("Error decoding sbatch template:", err)
		return ""
	}
	templateSbatch := string(templateSbatchBytes)

	jobName := fmt.Sprintf("akoflow_%d_%d", workflow.GetId(), activity.GetId())

	output := fmt.Sprintf("%s/akoflow_out_%d_%d.out", m.runtime.GetCurrentRuntimeMetadata("MOUNT_PATH"), workflow.GetId(), activity.GetId())
	error := fmt.Sprintf("%s/akoflow_err_%d_%d.err", m.runtime.GetCurrentRuntimeMetadata("MOUNT_PATH"), workflow.GetId(), activity.GetId())
	time := m.runtime.GetCurrentRuntimeMetadata("TIME")
	partition := m.runtime.GetCurrentRuntimeMetadata("QUEUE")
	ntasks := m.runtime.GetCurrentRuntimeMetadata("NTASKS")
	nodes := m.runtime.GetCurrentRuntimeMetadata("NODES")
	gpus := m.runtime.GetCurrentRuntimeMetadata("GPUS")
	cpusPerGpu := m.runtime.GetCurrentRuntimeMetadata("CPUS_PER_GPU")
	mem := m.runtime.GetCurrentRuntimeMetadata("MEM")
	wrap := m.GetSingularityCommand()

	templateSbatch = strings.ReplaceAll(templateSbatch, "#JOB_NAME#", jobName)
	templateSbatch = strings.ReplaceAll(templateSbatch, "#OUTPUT#", output)
	templateSbatch = strings.ReplaceAll(templateSbatch, "#ERROR#", error)
	templateSbatch = strings.ReplaceAll(templateSbatch, "#TIME#", time)
	templateSbatch = strings.ReplaceAll(templateSbatch, "#PARTITION#", partition)
	templateSbatch = strings.ReplaceAll(templateSbatch, "#NTASKS#", ntasks)
	templateSbatch = strings.ReplaceAll(templateSbatch, "#NODES#", nodes)
	templateSbatch = strings.ReplaceAll(templateSbatch, "#GPUS#", gpus)
	templateSbatch = strings.ReplaceAll(templateSbatch, "#CPUS_PER_GPU#", cpusPerGpu)
	templateSbatch = strings.ReplaceAll(templateSbatch, "#MEM#", mem)
	templateSbatch = strings.ReplaceAll(templateSbatch, "#COMMAND#", wrap)

	templateSbatchBase64 := base64.StdEncoding.EncodeToString([]byte(templateSbatch))

	templateSbatch = fmt.Sprintf("echo %s | base64 -d", templateSbatchBase64)

	command := templateSbatch + "| sbatch"

	commandBase64 := base64.StdEncoding.EncodeToString([]byte(command))

	command = fmt.Sprintf("echo %s | base64 -d | bash", commandBase64)

	return command
}
