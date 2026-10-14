package execution

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type attemptHandleStore struct {
	handles map[string]domain.ActivityHandle
}

func (s *attemptHandleStore) Find(_ context.Context, id string) (*domain.ActivityHandle, error) {
	h, ok := s.handles[id]
	if !ok {
		return nil, nil
	}
	return &h, nil
}

func (s *attemptHandleStore) Save(_ context.Context, h domain.ActivityHandle) error {
	s.handles[h.ID] = h
	return nil
}

func TestRecoveryStartsNewHandleAndKeepsSameAttemptIdempotent(t *testing.T) {
	prior := domain.ActivityHandle{ID: "run:activity", RunID: "run", ActivityID: "activity", Status: domain.HandleFailed, Failure: "old transfer error"}
	store := &attemptHandleStore{handles: map[string]domain.ActivityHandle{prior.ID: prior}}
	runtime := &runtimeFake{handle: domain.ActivityHandle{RunID: "run", ActivityID: "activity", RuntimeID: "local", Status: domain.HandleRunning}}
	controller := New(resolverFake{adapter: runtime}, store)
	execution := executionFixture()
	execution.Attempt = 2
	for i := 0; i < 2; i++ {
		h, err := controller.Start(context.Background(), execution)
		if err != nil || h.ID != "run:activity:attempt-2" || h.Status != domain.HandleRunning {
			t.Fatalf("recovery returned stale handle: %+v, %v", h, err)
		}
	}
	if runtime.starts != 1 || store.handles[prior.ID].Failure != prior.Failure {
		t.Fatalf("lost history or repeated command: starts=%d handles=%+v", runtime.starts, store.handles)
	}
}
