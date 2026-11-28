package hubpublish

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

type publicationVersionInput struct {
	YAML        string `json:"yaml"`
	Message     string `json:"message"`
	BaseVersion int    `json:"baseVersion"`
}

func (m *Manager) PreviewWorkflow(ctx context.Context, workflowID, publicationID, message string) (Preview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result Preview
	if _, err := m.identityLocked(ctx); err != nil {
		return result, err
	}
	destination, err := m.publicationLocked(ctx, publicationID)
	if err != nil {
		return result, err
	}
	if workflowID == "" || len(message) > 1000 {
		return result, failure(422, "invalid_request", "Choose a workflow and a version message up to 1000 characters.")
	}
	content, err := m.sourceRequest(ctx, http.MethodGet, "/workflow-definitions/"+url.PathEscape(workflowID)+"/export/", nil, "")
	if err != nil {
		return result, err
	}
	if len(content) > packageLimit {
		content, err = compactWorkflowYAML(content)
		if err != nil {
			return result, failure(422, "invalid_workflow", "The portable workflow could not be encoded.")
		}
	}
	if len(content) > packageLimit {
		return result, failure(422, "workflow_too_large", "The portable workflow exceeds 8 MiB.")
	}
	input := publicationVersionInput{YAML: string(content), Message: message}
	for _, version := range destination.Versions {
		input.BaseVersion = max(input.BaseVersion, version.Number)
	}
	var review json.RawMessage
	path := "/desktop/publications/" + url.PathEscape(publicationID) + "/versions/preview"
	if err := m.requestLocked(ctx, "POST", path, input, nil, true, &review); err != nil {
		return result, err
	}
	m.expirePreviewsLocked()
	if len(m.previews) >= 32 {
		return result, failure(429, "preview_limit", "Too many pending publication reviews.")
	}
	id, err := randomID()
	if err != nil {
		return result, err
	}
	m.previews[id] = preparedPublication{VersionInput: &input, Publication: destination.Publication, TokenHash: digest([]byte(m.state.AccessToken)), ExpiresAt: time.Now().Add(15 * time.Minute)}
	return Preview{ID: id, Publication: destination.Publication, WorkflowName: workflowID, YAML: input.YAML, Review: review, Logs: []map[string]any{}, Files: []any{}, ExportedAt: time.Now().UTC()}, nil
}

func (m *Manager) servePreviewWorkflow(w http.ResponseWriter, r *http.Request) {
	var input struct {
		WorkflowID    string `json:"workflowId"`
		PublicationID string `json:"publicationId"`
		Message       string `json:"message,omitempty"`
	}
	if err := decodeRequest(w, r, &input); err != nil {
		respond(w, nil, err)
		return
	}
	value, err := m.PreviewWorkflow(r.Context(), input.WorkflowID, input.PublicationID, input.Message)
	respond(w, value, err)
}
