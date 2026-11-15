package hubpublish

import "net/http"

func (m *Manager) serveWorkflowDefinition(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URI string `json:"uri"`
	}
	if err := decodeRequest(w, r, &input); err != nil {
		respond(w, nil, err)
		return
	}
	value, err := m.WorkflowDefinition(r.Context(), input.URI)
	respond(w, value, err)
}
