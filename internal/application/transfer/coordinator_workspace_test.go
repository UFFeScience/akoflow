package transfer

import (
	"context"
	"errors"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type workspaceSyncStub struct {
	runs   []domain.DataTransferRun
	err    error
	called int
}

func (s *workspaceSyncStub) Sync(_ context.Context, _ []domain.DataTransferPlan) ([]domain.DataTransferRun, error) {
	s.called++
	return s.runs, s.err
}

func TestWorkspacePreparationBlocksActivityOnFailedPredecessorSync(t *testing.T) {
	plan := domain.DataTransferPlan{ID: "edge", ExecutionRunID: "run", ProducerActivityID: "producer", ConsumerActivityID: "consumer", SyncWorkspace: true}
	requirement := domain.PreparationRequirement{Workspace: &domain.WorkspaceMaterialization{ID: "workspace", RevisionID: "run", Status: domain.MaterializationPlanned}, WorkspaceTransfers: []domain.DataTransferPlan{plan}}
	syncer := &workspaceSyncStub{runs: []domain.DataTransferRun{{ID: "edge", ExecutionRunID: "run", Status: domain.TransferFailed, Error: "ssh disconnected"}}, err: errors.New("ssh disconnected")}
	catalog := &materializationCatalogStub{}
	gate, err := (Coordinator{Catalog: catalog, WorkspaceSyncer: syncer}).Prepare(context.Background(), "consumer", requirement)
	if err == nil || gate != nil {
		t.Fatalf("successor released after transfer failure: gate=%+v err=%v", gate, err)
	}
	if syncer.called != 1 || len(catalog.runs) != 2 || catalog.runs[1].Status != domain.TransferFailed {
		t.Fatalf("transfer failure was not persisted: %+v", catalog.runs)
	}
}
