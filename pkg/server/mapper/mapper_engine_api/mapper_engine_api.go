package mapper_engine_api

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/mapper"
	"github.com/ovvesley/akoflow/pkg/server/types/types_api"
)

func MapEngineWorkflowEntityToApiWorkflowEntity(workflow workflow_entity.Workflow) types_api.ApiWorkflowType {
	apiWorkflow := types_api.ApiWorkflowType{}
	mapper.MapStructs(workflow, &apiWorkflow)
	return apiWorkflow
}

func MapEngineWorkflowEntityToApiWorkflowEntityList(workflow []workflow_entity.Workflow) []types_api.ApiWorkflowType {
	var apiWorkflow []types_api.ApiWorkflowType
	for _, wf := range workflow {
		apiWorkflow = append(apiWorkflow, MapEngineWorkflowEntityToApiWorkflowEntity(wf))
	}
	return apiWorkflow
}
