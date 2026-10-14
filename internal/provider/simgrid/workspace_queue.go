package simgrid

import "github.com/UFFeScience/akoflow/internal/domain"

// The native trace reports preparation separately. Waiting for a lane ends
// when preparation starts, not when computation starts after the merge.
func normalizeWorkspaceQueue(trace *domain.ExecutionTrace) {
	trace.Executed.QueueSeconds = 0
	for i := range trace.Tasks {
		task := &trace.Tasks[i]
		if source, _ := task.Metadata["workspaceMaterializationModelSource"].(string); source != "" {
			start, hasStart := task.Metadata["workspacePreparationStartedAt"].(float64)
			ready, hasReady := task.Metadata["workspaceRouteReadyAt"].(float64)
			if hasStart && hasReady {
				task.QueueSeconds = max(0, start-ready)
			}
		}
		trace.Executed.QueueSeconds += task.QueueSeconds
	}
}
