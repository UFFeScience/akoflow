package execution

import (
	"context"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type blockingExecutionPlan struct {
	started chan struct{}
	release chan struct{}
}

func (p *blockingExecutionPlan) Execute(_ context.Context, request ports.ExecutionRequest) (domain.ExecutionTrace, error) {
	close(p.started)
	<-p.release
	return domain.ExecutionTrace{RunID: request.Run.ID}, nil
}

func TestSupervisorRejectsConcurrentCommandWithoutMutatingRun(t *testing.T) {
	store := &executionStoreFake{}
	plan := &blockingExecutionPlan{started: make(chan struct{}), release: make(chan struct{})}
	service, err := New(store, &activityControllerFake{}, plan, Config{})
	if err != nil {
		t.Fatal(err)
	}
	request := requestFixture(domain.ExecutionModeSimulation)
	done := make(chan error, 1)
	go func() {
		_, err := service.Execute(context.Background(), request)
		done <- err
	}()
	<-plan.started
	_, err = service.Execute(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "active supervisor") || len(store.runs) != 1 || store.failed != "" {
		t.Fatalf("duplicate command mutated original execution: err=%v runs=%d failed=%q", err, len(store.runs), store.failed)
	}
	close(plan.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
