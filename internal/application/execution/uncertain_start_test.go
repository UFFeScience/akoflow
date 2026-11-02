package execution

import (
	"context"
	"errors"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func TestUncertainStartPersistsAndFencesDuplicateAttempt(t *testing.T) {
	runtime := &runtimeFake{startErr: ports.NewRuntimeReconciliationError(errors.New("lost reply"), map[string]any{"identity": "attempt"})}
	store := &handlesFake{}
	controller := New(resolverFake{adapter: runtime}, store)
	handle, err := controller.Start(context.Background(), executionFixture())
	if err == nil || handle.Status != domain.HandleStarting || store.handle == nil {
		t.Fatalf("uncertain start lost: %+v %v", handle, err)
	}
	_, err = controller.Start(context.Background(), executionFixture())
	if err == nil || runtime.starts != 1 {
		t.Fatal("duplicate attempt must remain fenced")
	}
	runtime.stopErr = errors.New("cleanup unconfirmed")
	if controller.Stop(context.Background(), handle.ID, domain.ExecutionModeReal) == nil {
		t.Fatal("unconfirmed cleanup accepted")
	}
	if store.handle.Status != domain.HandleStarting || store.handle.Metadata[ports.RuntimeReconciliationPending] != true {
		t.Fatal("cleanup fence lost")
	}
	runtime.stopErr = nil
	if err := controller.Stop(context.Background(), handle.ID, domain.ExecutionModeReal); err != nil {
		t.Fatal(err)
	}
	if store.handle.Status != domain.HandleStopped || store.handle.Metadata[ports.RuntimeReconciliationPending] == true {
		t.Fatal("confirmed cleanup not persisted")
	}
}
