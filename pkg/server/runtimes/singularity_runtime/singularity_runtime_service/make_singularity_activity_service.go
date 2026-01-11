package singularity_runtime_service

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type MakeSingularityActivityService struct {
}

func NewMakeSingularityActivityService() MakeSingularityActivityService {
	return MakeSingularityActivityService{}
}

func (s *MakeSingularityActivityService) Handle(workflow workflow_entity.Workflow, activity workflow_activity_entity.WorkflowActivities) string {
	return s.makeContainerCommandActivity(workflow, activity)
}

func (s *MakeSingularityActivityService) makeContainerCommandActivity(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) string {

	mountPath := wf.GetMountPath()
	memoryLimit := wfa.MemoryLimit
	cpuLimit := wfa.CpuLimit
	imageSifPath := wf.Spec.Image
	command := wfa.Run
	commandBase64 := base64.StdEncoding.EncodeToString([]byte(command))
	commandFinal := "echo " + commandBase64 + " | base64 -d | sh"

	entryPoint := fmt.Sprintf("singularity exec --bind %s:%s --pwd %s --memory %s --cpus %s %s bash -c '%s'",
		mountPath,
		mountPath,
		mountPath,
		memoryLimit,
		cpuLimit,
		imageSifPath,
		commandFinal,
	)

	strOutFile := fmt.Sprintf("%s/akoflow_out%s_%s.out",
		mountPath,
		strconv.Itoa(wfa.WorkflowId),
		strconv.Itoa(wfa.Id),
	)

	strErrFile := fmt.Sprintf("%s/akoflow_err%s_%s.err",
		mountPath,
		strconv.Itoa(wfa.WorkflowId),
		strconv.Itoa(wfa.Id),
	)

	entryPoint = fmt.Sprintf("%s > %s 2> %s", entryPoint, strOutFile, strErrFile)

	return entryPoint
}

func (s *MakeSingularityActivityService) MakeContainerCommandActivityToHPC(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities) string {

	mountPath := wf.GetMountPath()
	imageSifPath := wf.Spec.Image
	command := wfa.Run
	commandBase64 := base64.StdEncoding.EncodeToString([]byte(command))
	commandFinal := "echo " + commandBase64 + " | base64 -d | sh"

	entryPoint := fmt.Sprintf("singularity exec --bind %s:%s --pwd %s %s bash -c '%s'",
		mountPath,
		mountPath,
		mountPath,
		imageSifPath,
		commandFinal,
	)

	return entryPoint
}
