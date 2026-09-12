package workflow

import (
	"context"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type workflowStoreStub struct{ version *domain.WorkflowVersion }

func (s workflowStoreStub) Create(context.Context, domain.WorkflowDefinition) error { return nil }
func (s workflowStoreStub) List(context.Context) ([]domain.WorkflowDefinition, error) {
	return nil, nil
}
func (s workflowStoreStub) Find(context.Context, string) (*domain.WorkflowDefinition, error) {
	return nil, nil
}
func (s workflowStoreStub) FindVersion(context.Context, string) (*domain.WorkflowVersion, error) {
	return s.version, nil
}

type expansionStoreStub struct{ values []domain.WorkflowExpansion }

func (s *expansionStoreStub) SaveExpansion(_ context.Context, value domain.WorkflowExpansion) (*domain.WorkflowExpansion, error) {
	s.values = append(s.values, value)
	return &value, nil
}
func (s *expansionStoreStub) FindExpansionByEvent(_ context.Context, version, event string) (*domain.WorkflowExpansion, error) {
	for i := range s.values {
		if s.values[i].WorkflowVersionID == version && s.values[i].SourceEventID == event {
			return &s.values[i], nil
		}
	}
	return nil, nil
}
func (s *expansionStoreStub) ListExpansions(context.Context, string, string) ([]domain.WorkflowExpansion, error) {
	return append([]domain.WorkflowExpansion{}, s.values...), nil
}

func TestExpansionCoordinatorPersistsRejectedDecisionAndReplaysIt(t *testing.T) {
	base := &domain.WorkflowVersion{ID: "v", Activities: []domain.Activity{{
		ID: "root", Name: "root", Kind: domain.ActivityKindTask,
		Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal},
		Command:      domain.ActivityCommand{Entrypoint: "true"},
	}}}
	store := &expansionStoreStub{}
	coordinator := ExpansionCoordinator{Workflows: workflowStoreStub{version: base}, Store: store}
	request := domain.ExpansionRequest{
		ID: "r", WorkflowVersionID: "v", SourceActivityID: "missing", SourceEventID: "event", Sequence: 1,
		Activities: []domain.ExpansionActivity{{Key: "child", Activity: domain.Activity{
			Name: "child", Kind: domain.ActivityKindTask,
			Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal},
			Command:      domain.ActivityCommand{Entrypoint: "true"},
		}}},
	}
	first, err := coordinator.Apply(context.Background(), request)
	if err != nil || first.Status != "rejected" || first.FailureReason == "" {
		t.Fatalf("expected persisted rejection, got %+v err=%v", first, err)
	}
	second, err := coordinator.Apply(context.Background(), request)
	if err != nil || second.ID != first.ID || len(store.values) != 1 {
		t.Fatalf("replay was not idempotent: %+v err=%v", second, err)
	}
}

func TestExpansionCoordinatorRejectedOutOfOrderEventDoesNotConsumeSequence(t *testing.T) {
	base := &domain.WorkflowVersion{ID: "v", Activities: []domain.Activity{{
		ID: "root", Name: "root", Kind: domain.ActivityKindTask,
		Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal},
		Command:      domain.ActivityCommand{Entrypoint: "true"},
	}}}
	store := &expansionStoreStub{}
	coordinator := ExpansionCoordinator{Workflows: workflowStoreStub{version: base}, Store: store}
	request := func(event, key string, sequence int) domain.ExpansionRequest {
		return domain.ExpansionRequest{
			ID: event, WorkflowVersionID: "v", SourceActivityID: "root", SourceEventID: event, Sequence: sequence,
			Activities: []domain.ExpansionActivity{{Key: key, Activity: domain.Activity{
				Name: key, Kind: domain.ActivityKindTask,
				Capabilities: []domain.ActivityCapability{domain.ActivityCapabilityReal},
				Command:      domain.ActivityCommand{Entrypoint: "true"},
			}}},
		}
	}

	rejected, err := coordinator.Apply(context.Background(), request("early", "early-child", 2))
	if err != nil || rejected.Status != "rejected" {
		t.Fatalf("expected out-of-order rejection, got %+v err=%v", rejected, err)
	}
	first, err := coordinator.Apply(context.Background(), request("first", "first-child", 1))
	if err != nil || first.Status != "applied" {
		t.Fatalf("expected sequence 1 to remain applicable, got %+v err=%v", first, err)
	}
	second, err := coordinator.Apply(context.Background(), request("second", "second-child", 2))
	if err != nil || second.Status != "applied" {
		t.Fatalf("expected corrected sequence 2 to apply, got %+v err=%v", second, err)
	}
}
