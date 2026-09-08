package execution

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

type allocatorStore struct {
	ports.CloudConfigurationStore
	ports.CloudOperationStore
	mu         sync.Mutex
	operations map[string]domain.CloudOperationRun
	instances  map[string]domain.CloudProvisionedInstance
}

func (s *allocatorStore) ListProvisionedInstances(context.Context, string) ([]domain.CloudProvisionedInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := make([]domain.CloudProvisionedInstance, 0, len(s.instances))
	for _, value := range s.instances {
		values = append(values, value)
	}
	return values, nil
}
func (s *allocatorStore) FindProvisionedInstance(_ context.Context, id string) (*domain.CloudProvisionedInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.instances[id]
	if !ok {
		return nil, nil
	}
	return &value, nil
}
func (s *allocatorStore) CreateCloudOperation(_ context.Context, value domain.CloudOperationRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations[value.ID] = value
	return nil
}
func (s *allocatorStore) UpdateCloudOperation(_ context.Context, value domain.CloudOperationRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations[value.ID] = value
	return nil
}
func (s *allocatorStore) FindCloudOperation(_ context.Context, id string) (*domain.CloudOperationRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.operations[id]
	if !ok {
		return nil, nil
	}
	return &value, nil
}
func (s *allocatorStore) ListCloudOperations(context.Context) ([]domain.CloudOperationRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := make([]domain.CloudOperationRun, 0, len(s.operations))
	for _, value := range s.operations {
		values = append(values, value)
	}
	return values, nil
}

type allocatorQueue struct {
	ports.QueueStore
	store *allocatorStore
	jobs  []domainqueue.Job
}

func (q *allocatorQueue) Publish(_ context.Context, job domainqueue.Job) (domainqueue.Job, error) {
	q.jobs = append(q.jobs, job)
	go func() {
		time.Sleep(time.Millisecond)
		q.store.mu.Lock()
		defer q.store.mu.Unlock()
		for id, operation := range q.store.operations {
			operation.Status, operation.InstanceID = "completed", operation.Request.InstanceID
			q.store.operations[id] = operation
			q.store.instances[operation.InstanceID] = domain.CloudProvisionedInstance{ID: operation.InstanceID, CapacityTargetID: operation.CapacityTargetID, EnvironmentID: operation.EnvironmentID, Status: "ready"}
		}
	}()
	return job, nil
}

func TestQueuedCloudAllocatorAssociatesDurableOperationWithActivity(t *testing.T) {
	store := &allocatorStore{operations: map[string]domain.CloudOperationRun{}, instances: map[string]domain.CloudProvisionedInstance{}}
	queue := &allocatorQueue{store: store}
	allocator := QueuedCloudAllocator{Cloud: store, Operations: store, Queue: queue, PollInterval: time.Millisecond}
	instance, err := allocator.Allocate(context.Background(), "run", "activity", domain.CloudCapacityTarget{ID: "target", EnvironmentID: "environment"})
	if err != nil || instance.Status != "ready" {
		t.Fatalf("instance=%#v err=%v", instance, err)
	}
	if len(queue.jobs) != 1 || queue.jobs[0].Category != domainqueue.CategoryInfrastructure || queue.jobs[0].Priority != 100 {
		t.Fatalf("jobs=%#v", queue.jobs)
	}
	operations, _ := store.ListCloudOperations(context.Background())
	if len(operations) != 1 || operations[0].ExecutionRunID != "run" || operations[0].ActivityID != "activity" || operations[0].InstanceID != instance.ID {
		t.Fatalf("operations=%#v", operations)
	}
}

func TestQueuedCloudAllocatorStartsAndValidatesStoppedInstance(t *testing.T) {
	store := &allocatorStore{
		operations: map[string]domain.CloudOperationRun{},
		instances: map[string]domain.CloudProvisionedInstance{
			"stopped": {
				ID: "stopped", CapacityTargetID: "target",
				EnvironmentID: "environment", Status: "stopped",
			},
		},
	}
	queue := &allocatorQueue{store: store}
	allocator := QueuedCloudAllocator{
		Cloud: store, Operations: store, Queue: queue, PollInterval: time.Millisecond,
	}
	instance, err := allocator.Allocate(
		context.Background(),
		"run",
		"activity",
		domain.CloudCapacityTarget{ID: "target", EnvironmentID: "environment"},
	)
	if err != nil || instance.ID != "stopped" || instance.Status != "ready" {
		t.Fatalf("instance=%#v err=%v", instance, err)
	}
	if len(queue.jobs) != 2 {
		t.Fatalf("jobs=%#v", queue.jobs)
	}
	operations, _ := store.ListCloudOperations(context.Background())
	kinds := map[string]bool{}
	for _, operation := range operations {
		kinds[operation.Kind] = true
	}
	if !kinds["start"] || !kinds["validate"] {
		t.Fatalf("operation kinds=%#v", kinds)
	}
}
