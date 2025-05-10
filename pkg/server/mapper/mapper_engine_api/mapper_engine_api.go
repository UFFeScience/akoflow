package mapper_engine_api

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/runtime_entity"
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

func MapEngineRuntimeEntityToApiRuntimeEntity(runtime runtime_entity.Runtime) types_api.ApiRuntimeType {
	apiRuntime := types_api.ApiRuntimeType{}
	mapper.MapStructs(runtime, &apiRuntime)
	return apiRuntime
}

func MapEngineRuntimeEntityToApiRuntimeEntityList(runtime []runtime_entity.Runtime) []types_api.ApiRuntimeType {
	var apiRuntime []types_api.ApiRuntimeType
	for _, rt := range runtime {
		apiRuntime = append(apiRuntime, MapEngineRuntimeEntityToApiRuntimeEntity(rt))
	}
	return apiRuntime
}
