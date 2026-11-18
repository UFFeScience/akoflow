package hubpublish

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

type profileExecutionResource struct {
	Architecture string `json:"architecture"`
	Runtime      string `json:"runtime"`
	OS           string `json:"os"`
	CPUCores     int    `json:"cpuCores,omitempty"`
	MemoryBytes  int64  `json:"memoryBytes,omitempty"`
}

type profileExecutionSummary struct {
	Date            string                     `json:"date"`
	Mode            string                     `json:"mode"`
	DurationSeconds float64                    `json:"durationSeconds"`
	MachineLabel    string                     `json:"machineLabel,omitempty"`
	Resources       []profileExecutionResource `json:"resources"`
}

type profileExecutionInput struct {
	ShareKey string                  `json:"shareKey"`
	Summary  profileExecutionSummary `json:"summary"`
}

type profileExecutionPreview struct {
	ID         string                  `json:"id"`
	Author     string                  `json:"author"`
	Visibility string                  `json:"visibility"`
	Summary    profileExecutionSummary `json:"summary"`
}

func (m *Manager) previewProfileExecution(ctx context.Context, runID, label string) (profileExecutionPreview, error) {
	var result profileExecutionPreview
	if m.options.ReadOnly || m.options.Source == nil {
		return result, failure(403, "source_unavailable", "Sharing is unavailable from this instance.")
	}
	label = strings.TrimSpace(label)
	if runID == "" || len(runID) > 160 || len(label) > 80 || strings.ContainsFunc(label, unicode.IsControl) {
		return result, failure(422, "invalid_profile_execution", "Choose a completed execution and a label up to 80 bytes.")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	identity, err := m.identityLocked(ctx)
	if err != nil {
		return result, err
	}
	var execution map[string]any
	if err := m.readSource(ctx, "/execution-runs/"+url.PathEscape(runID)+"/?view=monitor", &execution); err != nil {
		return result, err
	}
	summary, err := projectProfileExecution(execution, label)
	if err != nil {
		return result, err
	}
	m.expirePreviewsLocked()
	if len(m.previews) >= 32 {
		return result, failure(429, "preview_limit", "Too many pending reviews. Wait a few minutes and try again.")
	}
	if m.state.ProfileShareSecret == "" {
		secret, err := randomID()
		if err != nil {
			return result, err
		}
		m.state.ProfileShareSecret = secret
		if err := m.persistLocked(); err != nil {
			m.state.ProfileShareSecret = ""
			return result, err
		}
	}
	mac := hmac.New(sha256.New, []byte(m.state.ProfileShareSecret))
	_, _ = mac.Write([]byte(runID))
	input := profileExecutionInput{ShareKey: hex.EncodeToString(mac.Sum(nil)), Summary: summary}
	id, err := randomID()
	if err != nil {
		return result, err
	}
	m.previews[id] = preparedPublication{Profile: &input, TokenHash: digest([]byte(m.state.AccessToken)), ExpiresAt: time.Now().Add(15 * time.Minute)}
	return profileExecutionPreview{ID: id, Author: identity.Username, Visibility: "public", Summary: summary}, nil
}

func projectProfileExecution(execution map[string]any, label string) (profileExecutionSummary, error) {
	run, _ := execution["run"].(map[string]any)
	mode, _ := run["mode"].(string)
	rawFinished, _ := run["finishedAt"].(string)
	finished, _ := time.Parse(time.RFC3339Nano, rawFinished)
	duration := positiveMetric(run["makespanSeconds"])
	if run["status"] != "completed" || finished.IsZero() || duration <= 0 ||
		(mode != "real" && mode != "simulation" && mode != "interactive") {
		return profileExecutionSummary{}, failure(422, "profile_execution_incomplete", "Sharing requires a completed execution with recorded timing.")
	}
	environment := executionEnvironment(nil, execution)
	result := profileExecutionSummary{Date: finished.UTC().Format(time.DateOnly), Mode: mode, DurationSeconds: duration,
		MachineLabel: label, Resources: []profileExecutionResource{}}
	for _, resource := range environment.Resources {
		if resource.Source != "observed" {
			continue
		}
		runtime := resource.Runtime
		if runtime == "" {
			runtime = "unknown"
		}
		result.Resources = append(result.Resources, profileExecutionResource{
			Architecture: resource.Architecture, Runtime: runtime, OS: resource.OS,
			CPUCores: int(min(resource.CPUCores, 65535)), MemoryBytes: int64(min(resource.MemoryBytes, 1<<53)),
		})
		if len(result.Resources) == 32 {
			break
		}
	}
	return result, nil
}

func (m *Manager) confirmProfileExecutionLocked(ctx context.Context, input profileExecutionInput, acknowledgePublic bool) (Receipt, error) {
	if !acknowledgePublic {
		return Receipt{}, failure(422, "public_confirmation", "Confirm that you reviewed the summary shared on your public profile.")
	}
	var response struct {
		ID     string `json:"id"`
		Author string `json:"author"`
	}
	if err := m.requestLocked(ctx, http.MethodPost, "/desktop/profile-executions", input, nil, true, &response); err != nil {
		return Receipt{}, err
	}
	if response.ID == "" || response.Author == "" {
		return Receipt{}, failure(502, "hub_response", "Hub did not return a profile execution link.")
	}
	return Receipt{URL: m.options.BrowserURL + "/u/" + url.PathEscape(response.Author) + "/executions/" + url.PathEscape(response.ID), ResultCount: 1}, nil
}

func (m *Manager) serveProfileExecutionPreview(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RunID        string `json:"runId"`
		MachineLabel string `json:"machineLabel"`
	}
	if err := decodeRequest(w, r, &input); err != nil {
		respond(w, nil, err)
		return
	}
	result, err := m.previewProfileExecution(r.Context(), input.RunID, input.MachineLabel)
	respond(w, result, err)
}
