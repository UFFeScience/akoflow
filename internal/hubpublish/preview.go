package hubpublish

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
func (m *Manager) Preview(ctx context.Context, runID, publicationID, message string, selections ...*ContentSelection) (Preview, error) {
	return m.previewExecution(ctx, runID, publicationID, message, 0, false, selections...)
}

func (m *Manager) previewExecution(ctx context.Context, runID, publicationID, message string, targetVersion int, contribution bool, selections ...*ContentSelection) (Preview, error) {
	if targetVersion < 0 || (contribution && targetVersion == 0) {
		return Preview{}, failure(422, "invalid_target_version", "Choose a published workflow version.")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var result Preview
	if _, err := m.identityLocked(ctx); err != nil {
		return result, err
	}
	destination, err := m.executionDestinationLocked(ctx, publicationID, contribution)
	if err != nil {
		return result, err
	}
	capture, err := m.capture(ctx, runID)
	if err != nil {
		return result, err
	}
	if len(selections) > 0 && selections[0] != nil {
		capture, err = selectCapture(capture, *selections[0])
		if err != nil {
			return result, err
		}
	}
	base := 0
	foundTarget := false
	for _, version := range destination.Versions {
		base = max(base, version.Number)
		foundTarget = foundTarget || version.Number == targetVersion
	}
	if targetVersion > 0 && !foundTarget {
		return result, failure(422, "invalid_target_version", "Choose an existing version of this publication.")
	}
	if len(message) > 1000 {
		return result, failure(422, "invalid_message", "Use a version message up to 1000 characters.")
	}
	if source, ok := m.state.WorkflowSources[capture.bundle.Source["workflowVersionId"]]; ok &&
		source.HubOrigin == m.state.HubURL && source.PublicationID == publicationID && source.Version == targetVersion &&
		source.LocalHash == digest([]byte(capture.yaml)) {
		if err := m.usePinnedWorkflowYAMLLocked(ctx, &capture, source); err != nil {
			return result, err
		}
	}
	data, err := json.Marshal(capture.bundle)
	if err != nil {
		return result, err
	}
	input := importInput{Contribution: contribution, BundleFile: string(data), BaseVersion: base, TargetVersion: targetVersion, Message: message}
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
	m.previews[id] = preparedPublication{RunID: runID, Input: input, Publication: destination.Publication, TokenHash: digest([]byte(m.state.AccessToken)), ExpiresAt: time.Now().Add(15 * time.Minute)}
	return describeExecutionPreview(id, runID, targetVersion, destination.Publication, capture, review), nil
}

func describeExecutionPreview(id, runID string, targetVersion int, publication Publication, capture capturedExperiment, review json.RawMessage) Preview {
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
	environment, hasEnvironment := capture.execution["environmentSnapshot"].(publishedEnvironment)
	result := Preview{ID: id, TargetVersion: targetVersion, Publication: publication, RunID: runID, WorkflowName: name, YAML: capture.yaml, Logs: logs, Files: files, ExportedAt: capture.bundle.ExportedAt, Review: review}
	if hasEnvironment {
		result.EnvironmentSnapshot = &environment
	}
	return result
}
func (m *Manager) expirePreviewsLocked() {
	for id, item := range m.previews {
		if time.Now().After(item.ExpiresAt) {
			delete(m.previews, id)
		}
	}
}

func (m *Manager) usePinnedWorkflowYAMLLocked(ctx context.Context, capture *capturedExperiment, source WorkflowSource) error {
	path := fmt.Sprintf("/publications/%s/versions/%d/yaml", url.PathEscape(source.PublicationID), source.Version)
	data, status, _, err := m.readWorkflowYAML(ctx, m.state.HubURL+"/api/v1"+path, "")
	if err == nil && status == 404 {
		data, status, _, err = m.readWorkflowYAML(ctx, m.state.HubURL+"/api/v1/desktop"+path, m.state.AccessToken)
	}
	if err != nil {
		return err
	}
	if status != 200 || digest(data) != source.Hash {
		return failure(409, "workflow_source_changed", "The original Hub definition could not be verified. Choose a matching version or publish the definition separately.")
	}
	capture.yaml = string(data)
	for i := range capture.bundle.Files {
		if capture.bundle.Files[i].Path == "workflow.yaml" {
			capture.bundle.Files[i].Content = capture.yaml
			capture.bundle.Files[i].SHA256 = source.Hash
		}
	}
	return nil
}
