package workflow_engine_api_handler

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMonitorOmitsOutputButLegacyDetailPreservesIt(t *testing.T) {
	h := newTestHandler()
	h.executions = executionQueryStub{run: &domain.ExecutionRun{ID: "run", SchedulePlanID: "plan"},
		handles: []domain.ActivityHandle{{ID: "handle", Log: "large captured output", Artifacts: &domain.ArtifactManifest{SchemaVersion: 1}}}}
	for _, view := range []string{"monitor", "", "status", "logs"} {
		r := httptest.NewRequest(http.MethodGet, "/execution-runs/run/?view="+view, nil)
		r.SetPathValue("runId", "run")
		w := httptest.NewRecorder()
		h.GetExecution(w, r)
		require.Equal(t, http.StatusOK, w.Code)
		switch view {
		case "monitor":
			require.NotContains(t, w.Body.String(), "large captured output")
			require.NotContains(t, w.Body.String(), "schemaVersion")
		case "status":
			require.NotContains(t, w.Body.String(), "handles")
			require.NotContains(t, w.Body.String(), "activities")
		case "logs":
			require.Contains(t, w.Body.String(), "large captured output")
			require.NotContains(t, w.Body.String(), "schemaVersion")
		default:
			require.Contains(t, w.Body.String(), "large captured output")
			require.Contains(t, w.Body.String(), "schemaVersion")
		}
	}
}
