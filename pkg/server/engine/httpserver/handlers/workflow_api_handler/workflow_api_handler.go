package workflow_api_handler

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/services/find_workflow_api_service"
	"github.com/ovvesley/akoflow/pkg/server/services/list_workflows_api_service"
	"net/http"
	"strconv"
)

type WorkflowApiHandler struct {
	listWorkflowApiService *list_workflows_api_service.ListWorkflowsApiService
	findWorkflowApiService *find_workflow_api_service.FindWorkflowApiService
}

func New() *WorkflowApiHandler {
	return &WorkflowApiHandler{
		listWorkflowApiService: list_workflows_api_service.New(),
		findWorkflowApiService: find_workflow_api_service.New(),
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

func (h *WorkflowApiHandler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowIdStr := config.App().HttpHelper.GetUrlParam(r, "workflowId")
	workflowId, err := strconv.Atoi(workflowIdStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	workflow, err := h.findWorkflowApiService.FindWorkflowById(workflowId)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	config.App().HttpHelper.WriteJson(w, workflow)

}
