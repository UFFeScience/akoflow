package hubpublish

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func (m *Manager) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /akoflow-api/hub/workflow-definition/", m.serveWorkflowDefinition)
	mux.HandleFunc("POST /akoflow-api/hub/workflows/import/", m.serveImportWorkflow)
	mux.HandleFunc("POST /akoflow-api/hub/publications/workflow-preview/", m.servePreviewWorkflow)
	mux.HandleFunc("GET /akoflow-api/hub/status/", m.serveStatus)
	mux.HandleFunc("PUT /akoflow-api/hub/settings/", m.serveSettings)
	mux.HandleFunc("POST /akoflow-api/hub/authorization/", m.serveStart)
	mux.HandleFunc("POST /akoflow-api/hub/authorization/poll/", m.servePoll)
	mux.HandleFunc("DELETE /akoflow-api/hub/connection/", m.serveDisconnect)
	mux.HandleFunc("GET /akoflow-api/hub/publications/", m.servePublications)
	mux.HandleFunc("GET /akoflow-api/hub/publications/{publicationId}/", m.serveGetPublication)
	mux.HandleFunc("PATCH /akoflow-api/hub/publications/{publicationId}/", m.serveUpdatePublication)
	mux.HandleFunc("DELETE /akoflow-api/hub/publications/{publicationId}/", m.serveDeletePublication)
	mux.HandleFunc("GET /akoflow-api/hub/publications/{publicationId}/versions/", m.serveListVersions)
	mux.HandleFunc("GET /akoflow-api/hub/publications/{publicationId}/results/", m.serveListResults)
	mux.HandleFunc("GET /akoflow-api/hub/publications/{publicationId}/compare/", m.serveCompareVersions)
	mux.HandleFunc("GET /akoflow-api/hub/publications/{publicationId}/contribution-destination/", m.serveContributionDestination)
	mux.HandleFunc("POST /akoflow-api/hub/publications/", m.serveCreate)
	mux.HandleFunc("POST /akoflow-api/hub/publications/preview/", m.servePreview)
	mux.HandleFunc("POST /akoflow-api/hub/publications/confirm/", m.serveConfirm)
	return mux
}
func respond(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		var failure *apiError
		if !errors.As(err, &failure) {
			failure = &apiError{Status: 500, Code: "hub_storage", Message: "The daemon could not save the Hub connection. Try again."}
		}
		w.WriteHeader(failure.Status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": failure.Message, "code": failure.Code})
		return
	}
	_ = json.NewEncoder(w).Encode(value)
}
func decodeRequest(w http.ResponseWriter, r *http.Request, value any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil || decoder.Decode(new(any)) != io.EOF {
		return failure(422, "invalid_request", "Check the Hub request fields.")
	}
	return nil
}
func (m *Manager) serveStatus(w http.ResponseWriter, r *http.Request) {
	value, err := m.Status(r.Context())
	if err == nil {
		m.mu.Lock()
		source, ok := m.state.WorkflowSources[r.URL.Query().Get("workflowVersionId")]
		m.mu.Unlock()
		if ok && source.HubOrigin == value.HubURL {
			value.WorkflowSource = &WorkflowSource{PublicationID: source.PublicationID, Version: source.Version, Hash: source.Hash}
		}
	}
	respond(w, value, err)
}
func (m *Manager) serveSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Enabled bool   `json:"enabled"`
		HubURL  string `json:"hubUrl"`
	}
	if err := decodeRequest(w, r, &input); err != nil {
		respond(w, nil, err)
		return
	}
	value, err := m.SaveSettings(r.Context(), input.Enabled, input.HubURL)
	respond(w, value, err)
}
func (m *Manager) serveStart(w http.ResponseWriter, r *http.Request) {
	value, err := m.StartAuthorization(r.Context())
	respond(w, value, err)
}
func (m *Manager) servePoll(w http.ResponseWriter, r *http.Request) {
	value, err := m.PollAuthorization(r.Context())
	respond(w, value, err)
}
func (m *Manager) serveDisconnect(w http.ResponseWriter, r *http.Request) {
	value, err := m.Disconnect(r.Context())
	respond(w, value, err)
}
func (m *Manager) servePublications(w http.ResponseWriter, r *http.Request) {
	value, err := m.Publications(r.Context())
	respond(w, value, err)
}
func (m *Manager) serveCreate(w http.ResponseWriter, r *http.Request) {
	var input Publication
	if err := decodeRequest(w, r, &input); err != nil {
		respond(w, nil, err)
		return
	}
	value, err := m.CreatePublication(r.Context(), input)
	respond(w, value, err)
}
func (m *Manager) serveContributionDestination(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.identityLocked(r.Context()); err != nil {
		respond(w, nil, err)
		return
	}
	value, err := m.contributionDestinationLocked(r.Context(), r.PathValue("publicationId"))
	respond(w, value, err)
}

func (m *Manager) servePreview(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Contribution  bool              `json:"contribution"`
		TargetVersion int               `json:"targetVersion"`
		RunID         string            `json:"runId"`
		PublicationID string            `json:"publicationId"`
		Message       string            `json:"message"`
		Content       *ContentSelection `json:"content"`
	}
	if err := decodeRequest(w, r, &input); err != nil {
		respond(w, nil, err)
		return
	}
	value, err := m.previewExecution(r.Context(), input.RunID, input.PublicationID, input.Message, input.TargetVersion, input.Contribution, input.Content)
	respond(w, value, err)
}
func (m *Manager) serveConfirm(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PreviewID         string `json:"previewId"`
		AcknowledgePublic bool   `json:"acknowledgePublic"`
	}
	if err := decodeRequest(w, r, &input); err != nil {
		respond(w, nil, err)
		return
	}
	value, err := m.Confirm(r.Context(), input.PreviewID, input.AcknowledgePublic)
	respond(w, value, err)
}
