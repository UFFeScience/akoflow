package simgrid

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	"testing"
)

func TestWorkspaceMergeIsNotBilledAsQueueAndLegacyQueueIsPreserved(t *testing.T) {
	trace := domain.ExecutionTrace{Tasks: []domain.TaskExecution{
		{QueueSeconds: 12, Metadata: map[string]any{"workspaceMaterializationModelSource": "frozen", "workspaceRouteReadyAt": 10.0, "workspacePreparationStartedAt": 13.0, "workspacePreparationSeconds": 9.0}},
		{QueueSeconds: 7},
	}}
	normalizeWorkspaceQueue(&trace)
	if trace.Tasks[0].QueueSeconds != 3 || trace.Tasks[1].QueueSeconds != 7 || trace.Executed.QueueSeconds != 10 {
		t.Fatalf("queue classification=%+v", trace)
	}
	normalizeWorkspaceQueue(&trace)
	if trace.Executed.QueueSeconds != 10 {
		t.Fatal("normalization is not idempotent")
	}
}
