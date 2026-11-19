package hubpublish

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	apirequests "github.com/UFFeScience/akoflow/internal/api/requests"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func (m *Manager) ImportWorkflow(ctx context.Context, uri, name string) (json.RawMessage, error) {
	return m.importWorkflow(ctx, uri, name, "", "", nil)
}

func (m *Manager) importWorkflow(ctx context.Context, uri, name, expectedHash, editedYAML string, selected *ExecutionReference) (json.RawMessage, error) {
	if m.options.ReadOnly {
		return nil, failure(403, "read_only", "Read-only instances cannot import workflows.")
	}
	definition, err := m.WorkflowDefinition(ctx, uri)
	if err != nil {
		return nil, err
	}
	if expectedHash != "" && expectedHash != definition.Hash {
		return nil, failure(409, "workflow_source_changed", "The reviewed Hub definition changed. Reload the source before importing.")
	}
	if editedYAML != "" && expectedHash == "" {
		return nil, failure(422, "workflow_source_required", "Review a Hub version before importing an edited copy.")
	}
	referencePath, err := m.importReferencePath(ctx, definition, expectedHash, selected)
	if err != nil {
		return nil, err
	}
	yaml := definition.YAML
	if editedYAML != "" {
		yaml = editedYAML
	}
	path := "/workflow-definitions/import/"
	if name != "" {
		path += "?" + url.Values{"name": {name}}.Encode()
	}
	data, err := m.sourceRequest(ctx, http.MethodPost, path, []byte(yaml), "application/yaml")
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, failure(502, "source_invalid", "The daemon returned an invalid workflow import response.")
	}
	var imported domain.WorkflowDefinition
	publicationID, version := definition.PublicationID, definition.Version
	if version > 0 && json.Unmarshal(data, &imported) == nil && imported.Version.ID != "" {
		localYAML, err := apirequests.FromDomain(imported).YAML()
		if err != nil {
			return nil, err
		}
		m.mu.Lock()
		if m.state.WorkflowSources == nil {
			m.state.WorkflowSources = make(map[string]WorkflowSource)
		}
		localHash, executedHash := "", ""
		if editedYAML == "" {
			localHash = digest(localYAML)
			executedHash, err = workflowChecksum(imported, true)
			if err != nil {
				m.mu.Unlock()
				return nil, err
			}
		}
		m.state.WorkflowSources[imported.Version.ID] = WorkflowSource{
			PublicationID: publicationID, Version: version, Hash: definition.Hash, LocalHash: localHash,
			HubOrigin: definition.origin, URI: definition.URI, Reference: selected, ReferencePath: referencePath,
			ExecutedHash: executedHash,
		}
		saveErr := m.persistLocked()
		m.mu.Unlock()
		if saveErr != nil {
			return nil, saveErr
		}
	}
	return json.RawMessage(data), nil
}

func (m *Manager) importReferencePath(ctx context.Context, definition WorkflowDefinition, expectedHash string, selected *ExecutionReference) (string, error) {
	if selected == nil {
		return "", nil
	}
	if expectedHash == "" || definition.PublicationID == "" || len(selected.ExperimentID) == 0 ||
		len(selected.ExperimentID) > 128 || len(selected.RunID) == 0 || len(selected.RunID) > 128 {
		return "", failure(422, "invalid_execution_reference", "Select an execution from the reviewed Hub version.")
	}
	path := fmt.Sprintf("/publications/%s/versions/%d/execution-references", definition.PublicationID, definition.Version)
	var references []executionReferenceChoice
	if err := m.historyRequest(ctx, definition.origin, http.MethodGet, path, nil, &references); err != nil {
		return "", err
	}
	for _, reference := range references {
		if reference.ExecutionReference != *selected {
			continue
		}
		if strings.HasPrefix(reference.Path, "/u/") || strings.HasPrefix(reference.Path, "/o/") || strings.HasPrefix(reference.Path, "/a/") {
			return reference.Path, nil
		}
		return "", nil
	}
	return "", failure(422, "invalid_execution_reference", "This execution is no longer eligible. Choose another reference.")
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
		if status == http.StatusConflict {
			var conflict struct {
				ExistingID string `json:"existingId"`
			}
			if json.Unmarshal(response.data.Bytes(), &conflict) == nil && conflict.ExistingID != "" {
				return nil, &apiError{Status: status, Code: "workflow_exists", Message: "This workflow name is already in use.", ExistingID: conflict.ExistingID}
			}
		}
		return nil, failure(status, "workflow_source_rejected", "The workflow API rejected this request. Review the definition or choose another name.")
	}
	return response.data.Bytes(), nil
}

func (m *Manager) serveImportWorkflow(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URI          string              `json:"uri"`
		Name         string              `json:"name,omitempty"`
		ExpectedHash string              `json:"expectedHash,omitempty"`
		Reference    *ExecutionReference `json:"reference,omitempty"`
		YAML         string              `json:"yaml,omitempty"`
	}
	if err := decodeBoundedRequest(w, r, &input, 16<<20); err != nil {
		respond(w, nil, err)
		return
	}
	value, err := m.importWorkflow(r.Context(), input.URI, input.Name, input.ExpectedHash, input.YAML, input.Reference)
	respond(w, value, err)
}
