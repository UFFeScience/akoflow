package server_connector_workflow

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type IWorkflow interface {
	Create(host string, port string, base64FileContent string)
}

type Workflow struct {
	client *http.Client
}

func New() *Workflow {
	return &Workflow{
		client: &http.Client{},
	}
}

type RequestPostRunWorkflowConnector struct {
	Workflow string `json:"workflow"`
}

type ResponsePostRunWorkflowConnector struct {
	Workflow string `json:"workflow"`
	Message  string `json:"message"`
}

func (w *Workflow) Create(host string, port string, base64FileContent string) {

	payload := ResponsePostRunWorkflowConnector{
		Workflow: base64FileContent,
	}

	payloadJson, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "http://"+host+":"+port+"/akoflow-server/workflow/", bytes.NewBuffer(payloadJson))

	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)

	if err != nil {
		messageError := "error: " + err.Error()
		println(messageError)
	}

	defer resp.Body.Close()

	var result ResponsePostRunWorkflowConnector
	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		println("error:", err)
	}

	println("workflow:", result.Workflow)
	println("message:", result.Message)
}
