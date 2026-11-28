package hubpublish

import (
	"net/http"
	"net/url"
	"strconv"
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
	id := r.PathValue("publicationId")
	if r.URL.EscapedPath() != "/akoflow-api/hub/publications/"+url.PathEscape(id)+"/" {
		http.NotFound(w, r)
		return
	}
	err := m.DeletePublication(r.Context(), id)
	respond(w, map[string]bool{"deleted": err == nil}, err)
}
func (m *Manager) serveRetireVersion(w http.ResponseWriter, r *http.Request) {
	id, rawVersion := r.PathValue("publicationId"), r.PathValue("version")
	if r.URL.EscapedPath() != "/akoflow-api/hub/publications/"+url.PathEscape(id)+"/versions/"+url.PathEscape(rawVersion)+"/" {
		http.NotFound(w, r)
		return
	}
	version, versionErr := strconv.Atoi(rawVersion)
	replacement, replacementErr := strconv.Atoi(r.URL.Query().Get("replacement"))
	if versionErr != nil || replacementErr != nil {
		respond(w, nil, failure(422, "invalid_version_retirement", "Choose an older version and its verified replacement."))
		return
	}
	err := m.RetireVersion(r.Context(), id, version, replacement)
	respond(w, map[string]bool{"retired": err == nil}, err)
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
