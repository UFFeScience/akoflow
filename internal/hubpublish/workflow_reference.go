package hubpublish

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var workflowReferencePattern = regexp.MustCompile(`^/(u|o)/[a-z0-9][a-z0-9_.-]{1,63}/[a-z0-9][a-z0-9-]{1,63}:(latest|v?[1-9][0-9]*)$`)

func (m *Manager) workflowReference(ctx context.Context, reference string) (WorkflowDefinition, error) {
	reference = strings.TrimSpace(reference)
	if !workflowReferencePattern.MatchString(reference) {
		return WorkflowDefinition{}, failure(422, "invalid_workflow_uri", "Use /u/owner/workflow:latest or /u/owner/workflow:v1.")
	}
	m.mu.Lock()
	origin, token := m.state.HubURL, m.state.AccessToken
	m.mu.Unlock()
	if origin == "" {
		return WorkflowDefinition{}, failure(503, "hub_not_configured", "AkôFlow Hub is not configured for this installation.")
	}
	address := origin + "/api/v1/workflow-definitions/resolve?" + url.Values{"reference": {reference}}.Encode()
	data, status, version, err := m.readWorkflowYAML(ctx, address, "")
	if err == nil && status == 404 && token != "" {
		data, status, version, err = m.readWorkflowYAML(ctx, address, token)
	}
	if err != nil {
		return WorkflowDefinition{}, err
	}
	if status != 200 {
		return WorkflowDefinition{}, hubResponseError(status, data)
	}
	if version < 1 {
		return WorkflowDefinition{}, failure(502, "hub_response", "Hub did not identify the resolved workflow version.")
	}
	hash := sha256.Sum256(data)
	base, _, _ := strings.Cut(reference, ":")
	return WorkflowDefinition{URI: base + ":v" + strconv.Itoa(version), Version: version, YAML: string(data), Hash: hex.EncodeToString(hash[:])}, nil
}
