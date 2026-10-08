package workflow_engine_api_handler

import (
	"context"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	queue "github.com/UFFeScience/akoflow/internal/domain/queue"
	"net/http/httptest"
	"testing"
)

type orphanOperations struct {
	ports.CloudOperationStore
	op domain.CloudOperationRun
}

func (s *orphanOperations) FindCloudOperation(context.Context, string) (*domain.CloudOperationRun, error) {
	v := s.op
	return &v, nil
}
func (s *orphanOperations) UpdateCloudOperation(_ context.Context, v domain.CloudOperationRun) error {
	s.op = v
	return nil
}

type orphanQueue struct {
	QueueCancellationStore
	job *queue.Job
}

func (s orphanQueue) FindLatestByAggregate(context.Context, string, string, string) (*queue.Job, error) {
	return s.job, nil
}

type orphanExecution struct {
	ExecutionQuery
	status domain.ExecutionRunStatus
}

func (s orphanExecution) FindRun(context.Context, string) (*domain.ExecutionRun, error) {
	return &domain.ExecutionRun{Status: s.status}, nil
}

func TestReconcileOrphanCloudOperationGuards(t *testing.T) {
	for _, tc := range []struct {
		name   string
		job    *queue.Job
		status domain.ExecutionRunStatus
		want   int
	}{
		{"orphan", nil, domain.ExecutionRunFailed, 200},
		{"queue job exists", &queue.Job{}, domain.ExecutionRunFailed, 409},
		{"workflow active", nil, domain.ExecutionRunRunning, 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &orphanOperations{op: domain.CloudOperationRun{ID: "orphan", Status: "queued", Kind: "start", ExecutionRunID: "run"}}
			h := &Handler{cloudOperations: s, queue: orphanQueue{job: tc.job}, executions: orphanExecution{status: tc.status}}
			r := httptest.NewRequest("POST", "/", nil)
			r.SetPathValue("operationId", "orphan")
			w := httptest.NewRecorder()
			h.ReconcileOrphanCloudOperation(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d body=%s", w.Code, w.Body)
			}
			if tc.want == 200 && (s.op.Status != "cancelled" || s.op.FinishedAt == nil) {
				t.Fatal("orphan not terminal")
			}
			if tc.want != 200 && s.op.Status != "queued" {
				t.Fatal("live operation changed")
			}
		})
	}
}
