package workflow_engine_api_handler

import (
	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

// Replaying a failed real workflow can repeat commands and cloud side effects.
// A failed run must instead be resumed through the explicit recovery API.
func configureExecutionJobRetries(job *domainqueue.Job, mode domain.ExecutionMode) {
	if mode == domain.ExecutionModeReal || mode == domain.ExecutionModeInteractive {
		job.MaxAttempts = 1
	}
}
