package server_connector_workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client interface {
	Create(host string, port string, base64FileContent string) error
}

type Workflow struct {
	client *http.Client
}

func New() *Workflow {
	return &Workflow{
		client: &http.Client{},
	}
}

func NewWithClient(client *http.Client) *Workflow {
	if client == nil {
		client = http.DefaultClient
	}
	return &Workflow{client: client}
}

type RequestPostRunWorkflowConnector struct {
	Workflow string `json:"workflow"`
}

type ResponsePostRunWorkflowConnector struct {
	Workflow string `json:"workflow"`
	Message  string `json:"message"`
}

func (w *Workflow) Create(host string, port string, base64FileContent string) error {
	payload := RequestPostRunWorkflowConnector{
		Workflow: base64FileContent,
	}

	payloadJSON, _ := json.Marshal(payload) // a string-only DTO cannot fail JSON encoding

	req, err := http.NewRequest("POST", "http://"+host+":"+port+"/akoflow-server/workflow/", bytes.NewBuffer(payloadJSON))
	if err != nil {
		return fmt.Errorf("create workflow request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send workflow request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("workflow request failed with status %s", resp.Status)
	}

	var result ResponsePostRunWorkflowConnector
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode workflow response: %w", err)
	}
	return nil
}
