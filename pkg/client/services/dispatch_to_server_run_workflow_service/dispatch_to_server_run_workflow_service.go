package dispatch_to_server_run_workflow_service

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/ovvesley/akoflow/pkg/client/utils"
)

type DispatchToServerRunWorkflowService struct {
	host string
	port string
	file string
}

type RequestPostRunWorkflow struct {
	Workflow string `json:"workflow"`
}

type ResponsePostRunWorkflow struct {
	Workflow string `json:"workflow"`
	Message  string `json:"message"`
}

func New() *DispatchToServerRunWorkflowService {
	return &DispatchToServerRunWorkflowService{}
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

	// read file

	base64FileContent := d.getBase64FileContent(d.GetFile())

	d.sendToServer(base64FileContent)

}

func (d *DispatchToServerRunWorkflowService) getBase64FileContent(filePath string) string {
	fileContent := d.getFileContent(filePath)

	base64FileContent := base64.StdEncoding.EncodeToString([]byte(fileContent))
	return base64FileContent
}

func (d *DispatchToServerRunWorkflowService) getFileContent(filePath string) string {
	return utils.ReadFile(filePath)
}

func (d *DispatchToServerRunWorkflowService) sendToServer(base64FileContent string) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	payload := RequestPostRunWorkflow{
		Workflow: base64FileContent,
	}

	payloadJson, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "http://"+d.GetHost()+":"+d.GetPort()+"/akoflow-server/workflow/run/", bytes.NewBuffer(payloadJson))

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)

	if err != nil {
		messageError := "error: " + err.Error()
		println(messageError)
	}

	defer resp.Body.Close()

	var result ResponsePostRunWorkflow
	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		println("error:", err)
	}

	println("workflow:", result.Workflow)
	println("message:", result.Message)

}
