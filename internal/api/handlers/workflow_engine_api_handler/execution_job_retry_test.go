package workflow_engine_api_handler

import (
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

func TestExecutionJobRequiresExplicitRealRecovery(t *testing.T) {
	for _, mode := range []domain.ExecutionMode{domain.ExecutionModeReal, domain.ExecutionModeInteractive} {
		job := domainqueue.Job{MaxAttempts: 5}
		configureExecutionJobRetries(&job, mode)
		if job.MaxAttempts != 1 {
			t.Fatalf("mode %s can replay real side effects: %d attempts", mode, job.MaxAttempts)
		}
	}
}

func TestNonRealExecutionPreservesQueueRetryPolicy(t *testing.T) {
	job := domainqueue.Job{MaxAttempts: 5}
	configureExecutionJobRetries(&job, domain.ExecutionMode("simulation"))
	if job.MaxAttempts != 5 {
		t.Fatalf("simulation retry policy changed: %d", job.MaxAttempts)
	}
}
