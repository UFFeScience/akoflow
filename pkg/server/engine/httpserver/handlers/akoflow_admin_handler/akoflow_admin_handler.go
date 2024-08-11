package akoflow_admin_handler

import (
	"html/template"
	"net/http"
)

const PATH_TEMPLATE = "pkg/server/engine/httpserver/handlers/akoflow_admin_handler/akoflow_admin_handler_tmpl/"

func GetTemplate(name string) *template.Template {
	return template.Must(template.ParseFiles(PATH_TEMPLATE + name))
}

type AkoflowAdminHandler struct {
	homeTemplate *template.Template
}

func New() *AkoflowAdminHandler {
	return &AkoflowAdminHandler{}
}

func (h *AkoflowAdminHandler) Home(w http.ResponseWriter, r *http.Request) {
	h.homeTemplate = template.Must(template.ParseFiles(PATH_TEMPLATE + "home.tmpl.html"))
	err := h.homeTemplate.Execute(w, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}
