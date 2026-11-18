package hubpublish

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/google/uuid"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var workflowReferencePattern = regexp.MustCompile(`^/(u|o|a)/[a-z0-9][a-z0-9_.-]{1,63}/[a-z0-9][a-z0-9-]{1,63}:(latest|v?[1-9][0-9]*)$`)

func (m *Manager) workflowReference(ctx context.Context, reference string) (WorkflowDefinition, error) {
	reference = strings.TrimSpace(reference)
	if strings.HasPrefix(reference, "akoflow::") {
		reference = strings.TrimPrefix(reference, "akoflow::")
		if strings.HasPrefix(reference, "o/") || strings.HasPrefix(reference, "a/") {
			reference = "/" + reference
		} else {
			reference = "/u/" + reference
		}
	}
	if !workflowReferencePattern.MatchString(reference) {
		return WorkflowDefinition{}, failure(422, "invalid_workflow_uri", "Use akoflow::owner/workflow:latest or akoflow::owner/workflow:v1.")
	}
	m.mu.Lock()
	origin, token := m.state.HubURL, m.state.AccessToken
	m.mu.Unlock()
	if origin == "" {
		return WorkflowDefinition{}, failure(503, "hub_not_configured", "AkôFlow Hub is not configured for this installation.")
	}
	address := origin + "/api/v1/workflow-definitions/resolve?" + url.Values{"reference": {reference}}.Encode()
	data, status, headers, err := m.readWorkflowYAML(ctx, address, "")
	if err == nil && status == 404 && token != "" {
		data, status, headers, err = m.readWorkflowYAML(ctx, address, token)
	}
	if err != nil {
		return WorkflowDefinition{}, err
	}
	if status != 200 {
		return WorkflowDefinition{}, hubResponseError(status, data)
	}
	version := headers.version
	if version < 1 {
		return WorkflowDefinition{}, failure(502, "hub_response", "Hub did not identify the resolved workflow version.")
	}
	hash := sha256.Sum256(data)
	base, _, _ := strings.Cut(reference, ":")
	publicationID := ""
	if id, err := uuid.Parse(headers.publicationID); err == nil {
		publicationID = id.String()
	}
	return WorkflowDefinition{
		PublicationID: publicationID,
		URI:           "akoflow::" + strings.TrimPrefix(strings.TrimPrefix(base, "/u/"), "/") + ":v" + strconv.Itoa(version),
		Version:       version, YAML: string(data), Hash: hex.EncodeToString(hash[:]),
	}, nil
}
