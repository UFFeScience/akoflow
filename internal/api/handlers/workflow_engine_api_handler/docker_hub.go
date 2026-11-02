package workflow_engine_api_handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/UFFeScience/akoflow/internal/infrastructure/dockerhub"
)

func writeDockerHubError(w http.ResponseWriter, err error) {
	var upstream *dockerhub.Error
	if errors.As(err, &upstream) {
		if upstream.RetryAfter != "" {
			w.Header().Set("Retry-After", upstream.RetryAfter)
		}
		writeError(w, upstream.Status, err)
		return
	}
	writeError(w, http.StatusBadGateway, fmt.Errorf("Docker Hub is unavailable. Try again."))
}
func dockerHubPage(r *http.Request) (int, error) {
	value := r.URL.Query().Get("page")
	if value == "" {
		return 1, nil
	}
	page, err := strconv.Atoi(value)
	if err != nil || page < 1 || page > 100 {
		return 0, fmt.Errorf("page must be between 1 and 100")
	}
	return page, nil
}
func (h *Handler) SearchDockerHub(w http.ResponseWriter, r *http.Request) {
	if h.dockerHub == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("Docker Hub is unavailable"))
		return
	}
	page, err := dockerHubPage(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := h.dockerHub.Search(r.Context(), r.URL.Query().Get("q"), page)
	if err != nil {
		writeDockerHubError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) DockerHubTags(w http.ResponseWriter, r *http.Request) {
	if h.dockerHub == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("Docker Hub is unavailable"))
		return
	}
	page, err := dockerHubPage(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := h.dockerHub.Tags(r.Context(), r.URL.Query().Get("repository"), r.URL.Query().Get("q"), page)
	if err != nil {
		writeDockerHubError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
