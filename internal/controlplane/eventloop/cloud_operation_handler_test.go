package eventloop

import (
	"context"
	"errors"
	"testing"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

type cloudOperationStoreStub struct {
	ports.CloudOperationStore
	operation domain.CloudOperationRun
	events    []domain.CloudOperationEvent
}

func (s *cloudOperationStoreStub) FindCloudOperation(context.Context, string) (*domain.CloudOperationRun, error) {
	value := s.operation
	return &value, nil
}

func (s *cloudOperationStoreStub) UpdateCloudOperation(_ context.Context, value domain.CloudOperationRun) error {
	s.operation = value
	return nil
}

func (s *cloudOperationStoreStub) AppendCloudOperationEvent(_ context.Context, value domain.CloudOperationEvent) error {
	s.events = append(s.events, value)
	return nil
}

type failingCloudProvisioner struct{ ports.CloudProvisioner }

func (failingCloudProvisioner) Provision(context.Context, string, domain.CloudProvisionRequest) (domain.CloudProvisionedInstance, error) {
	return domain.CloudProvisionedInstance{}, errors.New("temporary provider failure")
}

func (failingCloudProvisioner) Log(context.Context, string) ([]byte, error) { return nil, nil }

func TestCloudOperationRetryReturnsToDurableQueuedState(t *testing.T) {
	store := &cloudOperationStoreStub{operation: domain.CloudOperationRun{
		ID: "operation", Kind: "provision", Status: "queued", EnvironmentID: "environment",
	}}
	handler := NewCloudOperationHandler(store, failingCloudProvisioner{})
	job := domainqueue.Job{Payload: []byte(`{"operationId":"operation"}`), Attempts: 1, MaxAttempts: 3}
	if err := handler.Handle(context.Background(), job); err == nil {
		t.Fatal("expected retryable provider failure")
	}
	if store.operation.Status != "queued" || store.operation.Phase != "retrying" || store.operation.FinishedAt != nil {
		t.Fatalf("operation=%#v", store.operation)
	}
}
