package transfer

import (
	"context"
	"errors"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	executiondomain "github.com/UFFeScience/akoflow/internal/domain/execution"
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

func TestMixedDependencyRoutesPersistEvidenceBeforeRelease(t *testing.T) {
	plans := []domain.DataTransferPlan{
		{ID: "hpc-edge", ExecutionRunID: "run", ProducerActivityID: "hpc-producer", ConsumerActivityID: "consumer", SyncWorkspace: true},
		{ID: "fog-edge", ExecutionRunID: "run", ProducerActivityID: "fog-producer", ConsumerActivityID: "consumer", SyncWorkspace: true},
	}
	for _, test := range []struct {
		name         string
		secondStatus executiondomain.TransferStatus
		wantReady    bool
	}{
		{"all verified", domain.TransferCompleted, true},
		{"network still running", domain.TransferRunning, false},
		{"network failed", domain.TransferFailed, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			runs := []domain.DataTransferRun{
				{ID: "hpc-edge", Status: domain.TransferCompleted, NetworkBytes: 0, Route: domain.TransferRoute{SharedFilesystem: "lustre", SharedFilesystemID: "verified-id", ProducerActivityID: "hpc-producer"}},
				{ID: "fog-edge", Status: test.secondStatus, NetworkBytes: 100, Route: domain.TransferRoute{ProducerActivityID: "fog-producer"}},
			}
			catalog := &materializationCatalogStub{}
			requirement := domain.PreparationRequirement{Workspace: &domain.WorkspaceMaterialization{ID: "workspace", Status: domain.MaterializationPlanned}, WorkspaceTransfers: plans}
			gate, err := (Coordinator{Catalog: catalog, WorkspaceSyncer: &workspaceSyncStub{runs: runs}}).Prepare(context.Background(), "consumer", requirement)
			if (err == nil && gate != nil) != test.wantReady {
				t.Fatalf("mixed-route barrier: gate=%+v err=%v", gate, err)
			}
			if len(catalog.runs) != 4 || catalog.runs[2].Route.SharedFilesystemID != "verified-id" || catalog.runs[2].NetworkBytes != 0 || catalog.runs[3].NetworkBytes != 100 {
				t.Fatalf("route evidence not persisted: %+v", catalog.runs)
			}
		})
	}
}
