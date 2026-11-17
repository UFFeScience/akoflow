package workflow_engine_api_handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/UFFeScience/akoflow/internal/infrastructure/dockerhub"
	"github.com/UFFeScience/akoflow/internal/operationerror"
)

func writeDockerHubError(w http.ResponseWriter, err error) {
	var upstream *dockerhub.Error
	if errors.As(err, &upstream) {
		if upstream.RetryAfter != "" {
			w.Header().Set("Retry-After", upstream.RetryAfter)
		}
		code, retryable, action := "registry_unavailable", true, "Restore registry connectivity, then retry the lookup."
		switch upstream.Status {
		case http.StatusNotFound:
			code, retryable, action = "registry_image_not_found", false, "Choose an existing repository and image tag."
		case http.StatusTooManyRequests:
			code, action = "registry_rate_limited", "Wait for the registry retry interval before trying again."
		case http.StatusBadRequest:
			code, retryable, action = "registry_selection_invalid", false, "Correct the repository, tag and platform selection."
		}
		writeError(w, upstream.Status, operationerror.Wrap(err, code, "registry", "", retryable, action))
		return
	}
	writeError(w, http.StatusBadGateway, operationerror.Wrap(
		fmt.Errorf("Docker Hub is unavailable. Try again."), "registry_unavailable", "registry", "", true,
		"Restore registry connectivity, then retry the lookup.",
	))
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
