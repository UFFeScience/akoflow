package workflow_engine_api_handler

import "github.com/UFFeScience/akoflow/internal/domain"

func recoveryReasons(preview executionRecoveryPreview, tasks map[string]domain.TaskExecution) map[string]map[string]string {
	reasons := map[string]map[string]string{"reuse": {}, "retry": {}, "resume": {}, "cleanup": {}}
	for _, id := range preview.Reusable {
		reasons["reuse"][id] = "The previous attempt completed and its results can be reused."
	}
	for _, id := range preview.Failed {
		reason := tasks[id].FailureReason
		if reason == "" {
			reason = "The previous attempt failed."
		}
		reasons["retry"][id] = reason
	}
	for _, id := range preview.Interrupted {
		reason := "The previous attempt did not complete."
		if tasks[id].Status == domain.TaskCompleted {
			reason = "The completed output needed by another activity is unavailable."
		}
		reasons["resume"][id] = reason
	}
	for _, id := range preview.CleanupPending {
		reasons["cleanup"][id] = "The runtime or workspace still needs cleanup."
	}
	return reasons
}
