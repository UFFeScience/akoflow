package hubpublish

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"time"
)

func randomID() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}
func (m *Manager) Preview(ctx context.Context, runID, publicationID, message string) (Preview, error) {
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
	capture, err := m.capture(ctx, runID)
	if err != nil {
		return result, err
	}
	base := 0
	for _, version := range destination.Versions {
		base = max(base, version.Number)
	}
	if len(message) > 1000 {
		return result, failure(422, "invalid_message", "Use a version message up to 1000 characters.")
	}
	data, err := json.Marshal(capture.bundle)
	if err != nil {
		return result, err
	}
	input := importInput{BundleFile: string(data), BaseVersion: base, Message: message}
	var review json.RawMessage
	if err = m.requestLocked(ctx, "POST", "/desktop/publications/"+url.PathEscape(publicationID)+"/import/preview", input, nil, true, &review); err != nil {
		return result, err
	}
	id, err := randomID()
	if err != nil {
		return result, err
	}
	m.expirePreviewsLocked()
	if len(m.previews) >= 32 {
		return result, failure(429, "preview_limit", "Too many pending reviews. Wait a few minutes and try again.")
	}
	m.previews[id] = preparedPublication{Input: input, Publication: destination.Publication, TokenHash: digest([]byte(m.state.AccessToken)), ExpiresAt: time.Now().Add(15 * time.Minute)}
	logs := []map[string]any{}
	if records, ok := capture.execution["processLogs"].([]any); ok {
		for _, record := range records {
			if item, ok := record.(map[string]any); ok {
				logs = append(logs, item)
			}
		}
	}
	files, _ := capture.execution["files"].([]any)
	if files == nil {
		files = []any{}
	}
	name, _ := capture.workflow["name"].(string)
	result = Preview{ID: id, Publication: destination.Publication, RunID: runID, WorkflowName: name, YAML: capture.yaml, Logs: logs, Files: files, ExportedAt: capture.bundle.ExportedAt, Review: review}
	return result, nil
}
func (m *Manager) expirePreviewsLocked() {
	for id, item := range m.previews {
		if time.Now().After(item.ExpiresAt) {
			delete(m.previews, id)
		}
	}
}
