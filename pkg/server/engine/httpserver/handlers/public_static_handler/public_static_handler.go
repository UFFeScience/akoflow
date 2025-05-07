package public_static_handler

import (
	"html/template"
	"net/http"
)

const STATIC_PATH = "pkg/server/engine/httpserver/handlers/public_static_handler/public_static_handler_files/"

type PublicStaticHandler struct {
	homeTemplate *template.Template
}

func New() *PublicStaticHandler {
	return &PublicStaticHandler{}
}

func (h *PublicStaticHandler) Static(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", "/akoflow-admin/")
	w.WriteHeader(http.StatusFound)

}
