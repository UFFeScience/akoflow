package workflow_engine_api_handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestImportConflictPreservesExistingWorkflow(t *testing.T) {
	store := &workflowRepositoryStub{definition: &domain.WorkflowDefinition{ID: "study"}}
	handler := newTestHandler()
	handler.workflows = store
	request := httptest.NewRequest(http.MethodPost, "/workflow-definitions/import/", bytes.NewBufferString("name: study\nspec:\n  namespace: research\n  activities: []\n"))
	request.Header.Set("Content-Type", "application/yaml")
	recorder := httptest.NewRecorder()
	handler.CreateWorkflow(recorder, request)
	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"existingId":"study"`)
	require.Nil(t, store.created)
}

func TestImportCanUseSeparateLocalName(t *testing.T) {
	store := &workflowRepositoryStub{}
	handler := newTestHandler()
	handler.workflows = store
	request := httptest.NewRequest(http.MethodPost, "/workflow-definitions/import/?name=study-copy", bytes.NewBufferString("name: study\nspec:\n  namespace: research\n  activities: []\n"))
	request.Header.Set("Content-Type", "application/yaml")
	recorder := httptest.NewRecorder()
	handler.CreateWorkflow(recorder, request)
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	require.Equal(t, "study-copy", store.created.ID)
}
