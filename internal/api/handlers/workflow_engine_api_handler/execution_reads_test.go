package workflow_engine_api_handler

import (
	"context"
	"encoding/json"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainexecution "github.com/UFFeScience/akoflow/internal/domain/execution"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

type executionImagePreparationStub struct {
	executionQueryStub
	items []domainexecution.ImagePreparationObservation
}

func (reader executionImagePreparationStub) ListImagePreparation(context.Context, string) ([]domainexecution.ImagePreparationObservation, error) {
	return reader.items, nil
}

func TestMonitorExposesActiveImagePreparationWithoutReadingEventHistory(t *testing.T) {
	h := newTestHandler()
	start := time.Now().UTC().Add(-time.Second)
	progress := 0.25
	h.executions = executionImagePreparationStub{
		executionQueryStub: executionQueryStub{run: &domain.ExecutionRun{ID: "run", SchedulePlanID: "plan", StartedAt: &start}},
		items: []domainexecution.ImagePreparationObservation{{
			OperationID: "build-id", ActivityID: "activity", Attempt: 2,
			Kind: domainexecution.ImagePreparationBuild, Status: "running", StartedAt: start, Progress: &progress,
		}},
	}
	r := httptest.NewRequest(http.MethodGet, "/execution-runs/run/?view=monitor", nil)
	r.SetPathValue("runId", "run")
	w := httptest.NewRecorder()
	h.GetExecution(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	var response struct {
		Timeline domain.ExecutionTimeline
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Timeline.Phases, 1)
	phase := response.Timeline.Phases[0]
	require.Equal(t, "image-build", phase.Category)
	require.Equal(t, "build-id", phase.OperationID)
	require.Equal(t, "activity", phase.ActivityID)
	require.Equal(t, 2, phase.Attempt)
	require.True(t, phase.Active)
	require.Greater(t, phase.DurationSeconds, 0.0)
	require.NotNil(t, phase.Progress)
	require.Equal(t, progress, *phase.Progress)
}
