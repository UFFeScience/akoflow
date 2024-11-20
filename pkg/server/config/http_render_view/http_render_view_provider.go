package http_render_view

import (
	"fmt"
	"html/template"

	"github.com/ovvesley/akoflow/pkg/server/services/manipulation_files_service"
)

const PATH_TEMPLATE = "pkg/server/engine/httpserver/handlers/akoflow_admin_handler/akoflow_admin_handler_tmpl/"

type HttpRenderViewProvider struct {
	manipulationFilesService *manipulation_files_service.ManipulationFilesService
}

func New() HttpRenderViewProvider {
	return HttpRenderViewProvider{
		manipulationFilesService: manipulation_files_service.New(),
	}
}

func (p *HttpRenderViewProvider) makeTemplateInstance(path string) *template.Template {
	allTemplateFiles := manipulation_files_service.New().ListAllFilesInDir(PATH_TEMPLATE + "common/")
	allTemplateFiles = append([]string{PATH_TEMPLATE + path}, allTemplateFiles...)

	tmpl := template.New(path).Funcs(template.FuncMap{
		"dict": dict,
	})

	return template.Must(tmpl.ParseFiles(allTemplateFiles...))

}

func (p *HttpRenderViewProvider) TemplateInstance(templateName string) *template.Template {
	return p.makeTemplateInstance(templateName)
}

func dict(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict expects an even number of arguments")
	}
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		dict[key] = values[i+1]
	}
	return dict, nil
}
