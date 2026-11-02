package cloudprovision

import (
	"context"
	"errors"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type guardedReleaseStore struct {
	capacityStoreStub
	blocked error
	checked []string
}

func (s *guardedReleaseStore) CheckCloudLifecycle(_ context.Context, id, excluded string) error {
	s.checked = append(s.checked, id)
	if excluded != "" {
		return errors.New("direct release must not exclude an operation")
	}
	return s.blocked
}

func TestDirectReleaseHonorsLifecycleGuard(t *testing.T) {
	for _, policy := range []string{"destroy-after-run", "destroy-after-execution", "stop-when-idle"} {
		t.Run(policy, func(t *testing.T) {
			store := &guardedReleaseStore{capacityStoreStub: capacityStoreStub{
				target:    &domain.CloudCapacityTarget{ID: "target", EnvironmentID: "environment", LifecyclePolicy: policy},
				instances: []domain.CloudProvisionedInstance{{ID: "producer", CapacityTargetID: "target", Status: "ready"}},
			}, blocked: errors.New("producer still has consumers")}
			terraform := &terraformStub{}
			service := &Provisioner{store: store, terraform: terraform}
			if err := service.Release(context.Background(), []string{"target"}); err == nil {
				t.Fatal("unsafe release was accepted")
			}
			if len(store.checked) != 1 || len(terraform.destroyed) != 0 || len(terraform.stopped) != 0 || len(store.updated) != 0 {
				t.Fatal("release changed infrastructure before safety validation")
			}
		})
	}
}
