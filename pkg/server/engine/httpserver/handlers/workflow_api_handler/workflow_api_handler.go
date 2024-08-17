package workflow_api_handler

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/services/list_workflows_api_service"
	"net/http"
)

type WorkflowApiHandler struct {
	listWorkflowApiService *list_workflows_api_service.ListWorkflowsApiService
}

func New() *WorkflowApiHandler {
	return &WorkflowApiHandler{
		listWorkflowApiService: list_workflows_api_service.New(),
	}
}

func (h *WorkflowApiHandler) ListAllWorkflows(w http.ResponseWriter, r *http.Request) {

	workflows, err := h.listWorkflowApiService.ListAllWorkflows()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	config.App().HttpHelper.WriteJson(w, workflows)
}
