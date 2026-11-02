package workflow_engine_api_handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateExecutionRejectsExhaustedQuotaBeforeCreatingRun(t *testing.T) {
	// No execution command service is installed: reaching CreateRun would return
	// 503. The quota failure must instead be detected before any side effect.
	h := &Handler{}
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{
		"workflow":{"activities":[{"id":"a","metadata":{"workspaceProjection":{"contract":"whole-workspace-v1","source":"measured","inputs":[{"path":"input.fits","contentId":"hash","sizeBytes":100}]}}}]},
		"resources":[{"id":"node","metadata":{"workspaceStorageBudget":{"storageId":"home-quota","availableBytes":0,"observedAt":"2026-10-11T00:00:00Z","source":"quota-api"}}}],
		"plan":{"assignments":[{"activityId":"a","resourceId":"node"}]}
	}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	h.CreateExecution(response, request)
	require.Equal(t, http.StatusUnprocessableEntity, response.Code)
	require.Contains(t, response.Body.String(), "quota")
}
