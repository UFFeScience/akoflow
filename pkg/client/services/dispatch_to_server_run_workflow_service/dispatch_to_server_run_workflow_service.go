package dispatch_to_server_run_workflow_service

import (
	"encoding/base64"

	"github.com/ovvesley/akoflow/pkg/client/connector/server_connector"
	"github.com/ovvesley/akoflow/pkg/shared/utils/utils_read_file"
)

type DispatchToServerRunWorkflowService struct {
	host            string
	port            string
	file            string
	serverConnector server_connector.IServerConnector
}

func New(serverConnector server_connector.IServerConnector) *DispatchToServerRunWorkflowService {
	return &DispatchToServerRunWorkflowService{
		serverConnector: serverConnector,
	}
}

func (d *DispatchToServerRunWorkflowService) SetHost(host string) *DispatchToServerRunWorkflowService {
	d.host = host
	return d
}

func (d *DispatchToServerRunWorkflowService) SetPort(port string) *DispatchToServerRunWorkflowService {
	d.port = port
	return d
}

func (d *DispatchToServerRunWorkflowService) SetFile(file string) *DispatchToServerRunWorkflowService {
	d.file = file
	return d
}

func (d *DispatchToServerRunWorkflowService) GetHost() string {
	return d.host
}

func (d *DispatchToServerRunWorkflowService) GetPort() string {
	return d.port
}

func (d *DispatchToServerRunWorkflowService) GetFile() string {
	return d.file
}

func (d *DispatchToServerRunWorkflowService) Run() {

	println("host:", d.host)
	println("port:", d.port)
	println("file:", d.file)

	base64FileContent := d.getBase64FileContent(d.GetFile())

	d.sendToServer(base64FileContent)

}

func (d *DispatchToServerRunWorkflowService) getBase64FileContent(filePath string) string {
	fileContent := d.getFileContent(filePath)

	base64FileContent := base64.StdEncoding.EncodeToString([]byte(fileContent))
	return base64FileContent
}

func (d *DispatchToServerRunWorkflowService) getFileContent(filePath string) string {
	return utils_read_file.New().ReadFile(filePath)
}

func (d *DispatchToServerRunWorkflowService) sendToServer(base64FileContent string) {
	d.serverConnector.Workflow().Run(d.host, d.port, base64FileContent)
}
