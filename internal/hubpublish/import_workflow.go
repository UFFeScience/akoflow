package hubpublish

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

func (m *Manager) ImportWorkflow(ctx context.Context, uri, name string) (json.RawMessage, error) {
	if m.options.ReadOnly {
		return nil, failure(403, "read_only", "Read-only instances cannot import workflows.")
	}
	definition, err := m.WorkflowDefinition(ctx, uri)
	if err != nil {
		return nil, err
	}
	path := "/workflow-definitions/import/"
	if name != "" {
		path += "?" + url.Values{"name": {name}}.Encode()
	}
	data, err := m.sourceRequest(ctx, http.MethodPost, path, []byte(definition.YAML), "application/yaml")
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, failure(502, "source_invalid", "The daemon returned an invalid workflow import response.")
	}
	return json.RawMessage(data), nil
}

func (m *Manager) sourceRequest(ctx context.Context, method, path string, body []byte, contentType string) ([]byte, error) {
	if m.options.Source == nil {
		return nil, failure(503, "source_unavailable", "The workflow API is unavailable.")
	}
	request, err := http.NewRequestWithContext(ctx, method, "http://akoflow.internal/akoflow-api"+path, bytes.NewReader(body))
	if err != nil {
		return nil, failure(422, "source_invalid", "Choose a valid workflow.")
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response := &snapshotResponse{header: http.Header{}}
	m.options.Source.ServeHTTP(response, request)
	if response.status < 200 || response.status >= 300 || response.overflow {
		status := response.status
		if status < 400 || status > 599 {
			status = 502
		}
		return nil, failure(status, "workflow_source_rejected", "The workflow API rejected this request. Review the definition or choose another name.")
	}
	return response.data.Bytes(), nil
}

func (m *Manager) serveImportWorkflow(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URI  string `json:"uri"`
		Name string `json:"name,omitempty"`
	}
	if err := decodeRequest(w, r, &input); err != nil {
		respond(w, nil, err)
		return
	}
	value, err := m.ImportWorkflow(r.Context(), input.URI, input.Name)
	respond(w, value, err)
}
