package http_render_view

import (
	"fmt"
	"html/template"
	"path/filepath"

	"github.com/ovvesley/akoflow/pkg/server/services/manipulation_files_service"
	"github.com/ovvesley/akoflow/pkg/shared/utils/utils_read_file"
)

const PATH_TEMPLATE = "pkg/server/engine/httpserver/handlers/akoflow_admin_handler/akoflow_admin_handler_tmpl/"

func getPathTemplate() string {
	return filepath.Join(utils_read_file.New().GetRootProjectPath(), PATH_TEMPLATE) + "/"
}

type HttpRenderViewProvider struct {
	manipulationFilesService *manipulation_files_service.ManipulationFilesService
}

func New() HttpRenderViewProvider {
	return HttpRenderViewProvider{
		manipulationFilesService: manipulation_files_service.New(),
	}
}

func (p *HttpRenderViewProvider) makeTemplateInstance(path string) *template.Template {

	tmplFolder := getPathTemplate()

	allTemplateFiles := manipulation_files_service.New().ListAllFilesInDir(tmplFolder + "common/")
	allTemplateFiles = append([]string{tmplFolder + path}, allTemplateFiles...)

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
