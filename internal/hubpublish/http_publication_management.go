package hubpublish

import (
	"net/http"
	"net/url"
)

func (m *Manager) serveGetPublication(w http.ResponseWriter, r *http.Request) {
	value, err := m.ReadPublication(r.Context(), r.PathValue("publicationId"), "", nil)
	respond(w, value, err)
}
func (m *Manager) serveUpdatePublication(w http.ResponseWriter, r *http.Request) {
	var input PublicationPatch
	if err := decodeRequest(w, r, &input); err != nil {
		respond(w, nil, err)
		return
	}
	value, err := m.UpdatePublication(r.Context(), r.PathValue("publicationId"), input)
	respond(w, value, err)
}
func (m *Manager) serveDeletePublication(w http.ResponseWriter, r *http.Request) {
	err := m.DeletePublication(r.Context(), r.PathValue("publicationId"))
	respond(w, map[string]bool{"deleted": err == nil}, err)
}
func (m *Manager) serveListVersions(w http.ResponseWriter, r *http.Request) {
	value, err := m.ReadPublication(r.Context(), r.PathValue("publicationId"), "versions", nil)
	respond(w, value, err)
}
func (m *Manager) serveListResults(w http.ResponseWriter, r *http.Request) {
	value, err := m.ReadPublication(r.Context(), r.PathValue("publicationId"), "results", nil)
	respond(w, value, err)
}
func (m *Manager) serveCompareVersions(w http.ResponseWriter, r *http.Request) {
	query := url.Values{"base": {r.URL.Query().Get("base")}, "target": {r.URL.Query().Get("target")}}
	value, err := m.ReadPublication(r.Context(), r.PathValue("publicationId"), "compare", query)
	respond(w, value, err)
}
