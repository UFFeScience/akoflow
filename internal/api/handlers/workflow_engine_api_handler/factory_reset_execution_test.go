package workflow_engine_api_handler

import (
	"context"
	"github.com/UFFeScience/akoflow/internal/domain"
	"net/http"
	"testing"
)

func TestFactoryResetPreservesActiveExecutions(t *testing.T) {
	for _, status := range []domain.ExecutionRunStatus{domain.ExecutionRunCreated, domain.ExecutionRunRunning} {
		t.Run(string(status), func(t *testing.T) {
			called := false
			h := &Handler{executions: executionQueryStub{run: &domain.ExecutionRun{ID: "campaign", Status: status}}, factoryReset: func(context.Context) error { called = true; return nil }}
			response := callHandler(t, http.MethodPost, "/", "", nil, h.FactoryReset)
			if response.Code != http.StatusConflict || called {
				t.Fatalf("reset damaged active run: HTTP%d callback=%v", response.Code, called)
			}
		})
	}
}

func TestFactoryResetFailsClosedWithoutExecutionStateOrForArchives(t *testing.T) {
	for _, archived := range []bool{false, true} {
		called := false
		h := &Handler{readOnly: archived, factoryReset: func(context.Context) error { called = true; return nil }}
		response := callHandler(t, http.MethodPost, "/", "", nil, h.FactoryReset)
		want := http.StatusServiceUnavailable
		if archived {
			want = http.StatusForbidden
		}
		if response.Code != want || called {
			t.Fatalf("unsafe reset: HTTP%d callback=%v", response.Code, called)
		}
	}
}
