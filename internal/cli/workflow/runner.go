package workflow

import (
	"encoding/base64"
	"os"

	"github.com/UFFeScience/akoflow/internal/cli/api/server_connector"
)

type Runner struct {
	host            string
	port            string
	file            string
	serverConnector server_connector.Connector
}

func New(serverConnector server_connector.Connector) *Runner {
	return &Runner{
		serverConnector: serverConnector,
	}
}

func (d *Runner) SetHost(host string) *Runner {
	d.host = host
	return d
}

func (d *Runner) SetPort(port string) *Runner {
	d.port = port
	return d
}

func (d *Runner) SetFile(file string) *Runner {
	d.file = file
	return d
}

func (d *Runner) GetHost() string {
	return d.host
}

func (d *Runner) GetPort() string {
	return d.port
}

func (d *Runner) GetFile() string {
	return d.file
}

func (d *Runner) Run() error {
	base64FileContent, err := d.getBase64FileContent(d.GetFile())
	if err != nil {
		return err
	}
	return d.sendToServer(base64FileContent)
}

func (d *Runner) getBase64FileContent(filePath string) (string, error) {
	fileContent, err := d.getFileContent(filePath)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(fileContent), nil
}

func (d *Runner) getFileContent(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

func (d *Runner) sendToServer(base64FileContent string) error {
	return d.serverConnector.Workflow().Create(d.host, d.port, base64FileContent)
}
