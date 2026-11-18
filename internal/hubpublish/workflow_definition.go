package hubpublish

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type WorkflowDefinition struct {
	PublicationID string `json:"publicationId,omitempty"`
	URI           string `json:"uri"`
	Version       int    `json:"version"`
	YAML          string `json:"yaml"`
	Hash          string `json:"hash"`
}

func (m *Manager) WorkflowDefinition(ctx context.Context, uri string) (WorkflowDefinition, error) {
	if strings.HasPrefix(strings.TrimSpace(uri), "/") || strings.HasPrefix(strings.TrimSpace(uri), "akoflow::") {
		return m.workflowReference(ctx, uri)
	}
	id, version, err := parseWorkflowURI(uri)
	if err != nil {
		return WorkflowDefinition{}, err
	}
	m.mu.Lock()
	origin, token := m.state.HubURL, m.state.AccessToken
	m.mu.Unlock()
	if origin == "" {
		return WorkflowDefinition{}, failure(503, "hub_not_configured", "AkôFlow Hub is not configured for this installation.")
	}
	path := fmt.Sprintf("/publications/%s/versions/%d/yaml", id, version)
	data, status, _, err := m.readWorkflowYAML(ctx, origin+"/api/v1"+path, "")
	if err != nil {
		return WorkflowDefinition{}, err
	}
	if status == 404 && token != "" {
		data, status, _, err = m.readWorkflowYAML(ctx, origin+"/api/v1/desktop"+path, token)
	}
	if err != nil {
		return WorkflowDefinition{}, err
	}
	if status != 200 {
		if status == 404 {
			return WorkflowDefinition{}, failure(404, "workflow_not_found", "This workflow version is unavailable. For private publications, connect Desktop to its Hub project first.")
		}
		return WorkflowDefinition{}, hubResponseError(status, data)
	}
	hash := sha256.Sum256(data)
	return WorkflowDefinition{PublicationID: id, URI: fmt.Sprintf("akoflow://hub/publications/%s/versions/%d", id, version), Version: version, YAML: string(data), Hash: hex.EncodeToString(hash[:])}, nil
}

type resolvedDefinitionHeaders struct {
	version       int
	publicationID string
}

func (m *Manager) readWorkflowYAML(ctx context.Context, address, token string) ([]byte, int, resolvedDefinitionHeaders, error) {
	request, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return nil, 0, resolvedDefinitionHeaders{}, err
	}
	request.Header.Set("Accept", "application/yaml")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	client := http.Client{Timeout: 30 * time.Second}
	if m.options.Client != nil {
		client = *m.options.Client
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, resolvedDefinitionHeaders{}, failure(502, "hub_unavailable", "AkôFlow Hub cannot be reached. Try again.")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil || len(data) > 8<<20 || (response.StatusCode == 200 && len(data) == 0) {
		return nil, 0, resolvedDefinitionHeaders{}, failure(502, "hub_response", "The workflow YAML is unavailable or exceeds the definition size limit.")
	}
	version, _ := strconv.Atoi(response.Header.Get("X-Akoflow-Workflow-Version"))
	return data, response.StatusCode, resolvedDefinitionHeaders{
		version: version, publicationID: response.Header.Get("X-Akoflow-Publication-ID"),
	}, nil
}
