package execution

import "testing"

func TestReconcileWorkspaceRequestsAllForEmptyDestination(t *testing.T) {
	revision := WorkspaceRevision{ID: "r1", Blobs: []WorkspaceBlob{{Path: "a", Digest: "sha:a", SizeBytes: 1}, {Path: "b", Digest: "sha:b", SizeBytes: 2}}}
	missing := ReconcileWorkspace(revision, WorkspaceInventory{Digests: map[string]bool{}})
	if len(missing) != 2 {
		t.Fatalf("missing=%+v", missing)
	}
}
func TestReconcileWorkspaceRequestsOnlyMissingAndCommitVerifies(t *testing.T) {
	revision := WorkspaceRevision{ID: "r1", Blobs: []WorkspaceBlob{{Path: "a", Digest: "sha:a", SizeBytes: 1}, {Path: "b", Digest: "sha:b", SizeBytes: 2}, {Path: "same", Digest: "sha:b", SizeBytes: 2}}}
	m := WorkspaceMaterialization{ID: "m"}
	m.Reconcile(revision, WorkspaceInventory{Digests: map[string]bool{"sha:a": true}})
	if len(m.Missing) != 1 || m.Missing[0].Digest != "sha:b" {
		t.Fatalf("missing=%+v", m.Missing)
	}
	if err := m.Commit(nil); err == nil {
		t.Fatal("expected commit verification failure")
	}
	if err := m.Commit([]string{"sha:b"}); err != nil || m.Status != MaterializationCommitted {
		t.Fatalf("status=%s err=%v", m.Status, err)
	}
}
func TestPreparationGateBlocksIncompleteMaterialization(t *testing.T) {
	gate := PreparationGate{Workspace: &WorkspaceMaterialization{Status: MaterializationTransferring}}
	if gate.Ready() == nil {
		t.Fatal("expected materialization barrier")
	}
}
