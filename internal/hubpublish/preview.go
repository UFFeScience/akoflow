package hubpublish

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	apirequests "github.com/UFFeScience/akoflow/internal/api/requests"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func randomID() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}
func (m *Manager) Preview(ctx context.Context, runID, publicationID, message string, selections ...*ContentSelection) (Preview, error) {
	return m.previewExecution(ctx, runID, publicationID, message, 0, false, false, selections...)
}

func (m *Manager) previewExecution(ctx context.Context, runID, publicationID, message string, targetVersion int, contribution, fork bool, selections ...*ContentSelection) (Preview, error) {
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
	var selection *ContentSelection
	if len(selections) > 0 {
		selection = selections[0]
	}
	capture, err := m.capture(ctx, runID, selection)
	if err != nil {
		return result, err
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
	if err := m.checkExecutionSourceLocked(ctx, runID, &capture, publicationID, targetVersion, fork); err != nil {
		return result, err
	}
	if fork {
		if err := m.recordForkOriginLocked(&capture, publicationID, base, targetVersion); err != nil {
			return result, err
		}
	}
	data, err := marshalExperiment(capture.bundle)
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

func (m *Manager) recordForkOriginLocked(capture *capturedExperiment, publicationID string, base, targetVersion int) error {
	source, exists := m.state.WorkflowSources[capture.bundle.Source["workflowVersionId"]]
	if !exists || source.HubOrigin != m.state.HubURL || source.Version < 1 ||
		base != 0 || targetVersion != 0 || publicationID == source.PublicationID {
		return failure(409, "invalid_fork_destination", "Forks require a new publication and a recorded source on this Hub.")
	}
	capture.bundle.ForkOrigin = &forkOrigin{PublicationID: source.PublicationID, Version: source.Version, Hash: source.Hash}
	return nil
}

func (m *Manager) checkExecutionSourceLocked(ctx context.Context, runID string, capture *capturedExperiment, publicationID string, version int, fork bool) error {
	versionID := capture.bundle.Source["workflowVersionId"]
	source, exists := m.state.WorkflowSources[versionID]
	if !exists || fork {
		return nil
	}
	if version == 0 {
		return failure(409, "imported_execution_destination_required", "Share on the recorded Hub version or explicitly fork the workflow.")
	}
	executed, err := m.executedDefinition(ctx, runID, versionID)
	if err != nil {
		return err
	}
	expected := source.ExecutedHash
	if expected == "" {
		expected = source.LocalHash
	}
	actual, err := workflowChecksum(executed, source.ExecutedHash != "")
	if err != nil {
		return err
	}
	if expected == "" || actual != expected {
		return failure(409, "workflow_definition_mismatch", "The executed definition differs from the imported version. Fork the workflow to publish these changes.")
	}
	return m.verifyImportedDestinationLocked(ctx, capture, source, publicationID, version)
}

func (m *Manager) executedDefinition(ctx context.Context, runID, versionID string) (domain.WorkflowDefinition, error) {
	var definition domain.WorkflowDefinition
	path := "/execution-runs/" + url.PathEscape(runID) + "/?view=publication-definition"
	if err := m.readSource(ctx, path, &definition); err != nil {
		return definition, err
	}
	if definition.Version.ID != versionID {
		return definition, failure(409, "workflow_source_mismatch", "The recorded run belongs to a different workflow version.")
	}
	return definition, nil
}

func workflowChecksum(definition domain.WorkflowDefinition, canonical bool) (string, error) {
	document := apirequests.FromDomain(definition)
	for index := range document.Spec.Activities {
		command := &document.Spec.Activities[index].Command
		if command.Executable != nil && command.Executable.Source.Type == domain.ExecutableSourceOCI &&
			command.Image == command.Executable.Source.Reference && canonical {
			command.Image = ""
		}
	}
	yaml, err := document.YAML()
	if err != nil {
		return "", err
	}
	return digest(yaml), nil
}

func (m *Manager) verifyImportedDestinationLocked(ctx context.Context, capture *capturedExperiment, source WorkflowSource, publicationID string, version int) error {
	if source.HubOrigin != m.state.HubURL || source.PublicationID != publicationID || source.Version != version {
		return failure(409, "workflow_source_mismatch", "This execution was imported from a different Hub version. Share it on its recorded source version or fork the workflow.")
	}
	return m.usePinnedWorkflowYAMLLocked(ctx, capture, source)
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
