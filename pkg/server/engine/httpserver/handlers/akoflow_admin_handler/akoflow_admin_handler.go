package akoflow_admin_handler

import (
	"net/http"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/config/http_render_view"
)

type AkoflowAdminHandler struct {
	renderViewProvider http_render_view.HttpRenderViewProvider
}

func New() *AkoflowAdminHandler {
	return &AkoflowAdminHandler{
		renderViewProvider: config.App().TemplateRenderer.RenderViewProvider,
	}
}

func (h *AkoflowAdminHandler) Home(w http.ResponseWriter, r *http.Request) {
	homeTemplate := h.renderViewProvider.TemplateInstance("home.tmpl.html")
	err := homeTemplate.Execute(w, map[string]interface{}{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

func (h *AkoflowAdminHandler) WorkflowDetail(w http.ResponseWriter, r *http.Request) {
	workflowDetailTemplate := h.renderViewProvider.TemplateInstance("workflow_detail.tmpl.html")
	err := workflowDetailTemplate.Execute(w, map[string]interface{}{})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

func (h *AkoflowAdminHandler) Runtime(w http.ResponseWriter, r *http.Request) {
	runtimeTemplate := h.renderViewProvider.TemplateInstance("runtimes.tmpl.html")
	err := runtimeTemplate.Execute(w, map[string]interface{}{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *AkoflowAdminHandler) Schedule(w http.ResponseWriter, r *http.Request) {
	scheduleTemplate := h.renderViewProvider.TemplateInstance("schedules.tmpl.html")
	err := scheduleTemplate.Execute(w, map[string]interface{}{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
