package execution

import (
	"context"
	"testing"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type recoveryLeaseStore struct {
	ports.WorkspaceStore
	leases []domain.WorkspaceLease
	saved  []domain.WorkspaceLease
}

func (s *recoveryLeaseStore) ListWorkspaceLeases(context.Context, string) ([]domain.WorkspaceLease, error) {
	return s.leases, nil
}
func (s *recoveryLeaseStore) SaveWorkspaceLease(_ context.Context, lease domain.WorkspaceLease) error {
	s.saved = append(s.saved, lease)
	return nil
}

func TestRecoveryRejectsSealedButEmptyWorkspace(t *testing.T) {
	w := domain.ActivityWorkspace{State: domain.WorkspaceSealed, FileCount: 47, SizeBytes: 196938502}
	if recoveryWorkspacePresent(w, domain.WorkspaceUsage{}) {
		t.Fatal("stale manifest reused an empty producer")
	}
	if !recoveryWorkspacePresent(w, domain.WorkspaceUsage{FileCount: w.FileCount, SizeBytes: w.SizeBytes}) {
		t.Fatal("intact workspace rejected")
	}
}

func TestRecoveryRearmsOnlyPendingConsumerLeases(t *testing.T) {
	now := time.Now()
	store := &recoveryLeaseStore{leases: []domain.WorkspaceLease{
		{ID: "pending", ConsumerActivityID: "retry", Released: true, ReleasedAt: &now, ReleaseReason: "previous attempt"},
		{ID: "completed", ConsumerActivityID: "done", Released: true},
	}}
	if err := restoreRecoveryWorkspaceLeases(context.Background(), store, "run", map[string]bool{"done": true}); err != nil {
		t.Fatal(err)
	}
	if len(store.saved) != 1 || store.saved[0].ID != "pending" || store.saved[0].Released || store.saved[0].ReleasedAt != nil || store.saved[0].ReleaseReason != "" {
		t.Fatalf("incorrect retention: %+v", store.saved)
	}
}
