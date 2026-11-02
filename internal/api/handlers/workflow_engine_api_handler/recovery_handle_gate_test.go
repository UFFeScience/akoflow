package workflow_engine_api_handler

import (
	"context"
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

type recoveryHandleQuery struct {
	executionQueryStub
	handles []domain.ActivityHandle
}

func (s recoveryHandleQuery) ListHandles(context.Context, string) ([]domain.ActivityHandle, error) {
	return s.handles, nil
}

func TestRecoveryRejectsActiveHandlesFromEarlierAttempts(t *testing.T) {
	for _, status := range []domain.ActivityHandleStatus{domain.HandleStarting, domain.HandleRunning, domain.HandleStopped, domain.HandleCompleted, domain.HandleFailed} {
		h := &Handler{executions: recoveryHandleQuery{executionQueryStub: executionQueryStub{run: &domain.ExecutionRun{ID: "run", Status: domain.ExecutionRunFailed}}, handles: []domain.ActivityHandle{{ActivityID: "activity", Status: status}}}}
		preview, err := h.executionRecoveryPreview(context.Background(), "run")
		if err != nil {
			t.Fatal(err)
		}
		active := status == domain.HandleStarting || status == domain.HandleRunning
		if preview.Recoverable == active {
			t.Fatalf("status=%s preview=%+v", status, preview)
		}
		if active && (len(preview.CleanupPending) != 1 || preview.CleanupPending[0] != "activity") {
			t.Fatalf("missing cleanup reason: %+v", preview)
		}
	}
}
