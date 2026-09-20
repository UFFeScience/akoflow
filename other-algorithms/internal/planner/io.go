package planner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type APILoader struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func ReadInput(path string) (Input, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return Input{}, err
	}
	var input Input
	if err := yaml.Unmarshal(data, &input); err != nil {
		return Input{}, fmt.Errorf("decode planning input: %w", err)
	}
	return input, ValidateInput(input)
}

func WriteEnvelope(writer io.Writer, envelope ImportEnvelope, format string) error {
	switch strings.ToLower(format) {
	case "json":
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(envelope)
	case "yaml", "yml":
		encoder := yaml.NewEncoder(writer)
		encoder.SetIndent(2)
		defer encoder.Close()
		return encoder.Encode(envelope)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func (loader APILoader) Load(scopeID, workflowID, topologyID string) (Input, error) {
	if scopeID == "" || workflowID == "" || topologyID == "" {
		return Input{}, fmt.Errorf("scope, workflow, and topology IDs are required in API mode")
	}
	var scope ExecutionScope
	if err := loader.get("execution-scopes/"+url.PathEscape(scopeID)+"/", &scope); err != nil {
		return Input{}, err
	}
	var definition WorkflowDefinition
	if err := loader.get("workflow-definitions/"+url.PathEscape(workflowID)+"/", &definition); err != nil {
		return Input{}, err
	}
	var topology NetworkTopology
	if err := loader.get("network-topologies/"+url.PathEscape(topologyID)+"/", &topology); err != nil {
		return Input{}, err
	}
	var resources []Resource
	if err := loader.get("resources/", &resources); err != nil {
		return Input{}, err
	}
	input := Input{
		Workflow:        definition.Version,
		ExecutionScope:  scope,
		Resources:       resources,
		NetworkTopology: topology,
	}
	return input, ValidateInput(input)
}

func (loader APILoader) get(endpoint string, target any) error {
	client := loader.Client
	if client == nil {
		client = http.DefaultClient
	}
	request, err := http.NewRequest(http.MethodGet, strings.TrimRight(loader.BaseURL, "/")+"/"+endpoint, nil)
	if err != nil {
		return err
	}
	if loader.Token != "" {
		request.Header.Set("Authorization", "Bearer "+loader.Token)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("GET %s: %w", endpoint, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("GET %s returned %s: %s", endpoint, response.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode GET %s: %w", endpoint, err)
	}
	return nil
}
