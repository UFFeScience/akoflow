package workflow_engine_api_handler

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/infrastructure/dockerhub"
	"github.com/UFFeScience/akoflow/internal/operationerror"
)

func TestOperationalErrorRetainsMessageAndAddsSafeDetails(t *testing.T) {
	response := httptest.NewRecorder()
	writeError(response, 422, operationerror.Wrap(errors.New("registry token=unique-secret"), "registry_pull_failed", "artifact_preparation", "activity", true, "Check registry access."))
	var value struct {
		Error   string                 `json:"error"`
		Details operationerror.Details `json:"details"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if value.Error == "" || strings.Contains(response.Body.String(), "unique-secret") || value.Details.EntityID != "activity" || value.Details.CorrelationID != response.Header().Get("X-Akoflow-Error-ID") {
		t.Fatalf("invalid failure response: %s", response.Body.String())
	}
}

func TestRegistryRateLimitPreservesRetryHeaderAndRecoveryCode(t *testing.T) {
	response := httptest.NewRecorder()
	writeDockerHubError(response, &dockerhub.Error{Status: 429, RetryAfter: "60", Message: "Registry limit reached."})
	if response.Code != 429 || response.Header().Get("Retry-After") != "60" || !strings.Contains(response.Body.String(), "registry_rate_limited") {
		t.Fatalf("lost registry recovery: %d %s", response.Code, response.Body.String())
	}
}
