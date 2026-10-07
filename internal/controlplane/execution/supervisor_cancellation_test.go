package execution

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type cancelledExecutionPlan struct{}

func (cancelledExecutionPlan) Execute(ctx context.Context, _ ports.ExecutionRequest) (domain.ExecutionTrace, error) {
	return domain.ExecutionTrace{}, ctx.Err()
}

func TestSupervisorPersistsLeaseCancellationCause(t *testing.T) {
	store := &executionStoreFake{}
	service, err := New(store, &activityControllerFake{}, cancelledExecutionPlan{}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errors.New("renew lease for queue job example: lease ownership lost"))
	_, err = service.Execute(ctx, requestFixture(domain.ExecutionModeSimulation))
	if !errors.Is(err, context.Canceled) || !strings.Contains(store.failed, "lease ownership lost") {
		t.Fatalf("lease cancellation hidden from run: err=%v failure=%q", err, store.failed)
	}
}
