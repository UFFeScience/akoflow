package execution

import (
	"encoding/json"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/controlplane/eventloop"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

func NewExecutionJob(request ports.ExecutionRequest, key string, now time.Time) (domainqueue.Job, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return domainqueue.Job{}, err
	}
	job, err := domainqueue.New(domainqueue.CategoryExecution, eventloop.EventExecutionRunRequested, payload, now)
	if err != nil {
		return domainqueue.Job{}, err
	}
	job.AggregateType, job.AggregateID, job.IdempotencyKey = "execution_run", request.Run.ID, key
	ConfigureExecutionJobRetries(&job, request.Run.Mode)
	return job, nil
}

func ConfigureExecutionJobRetries(job *domainqueue.Job, mode domain.ExecutionMode) {
	if mode == domain.ExecutionModeReal || mode == domain.ExecutionModeInteractive {
		job.MaxAttempts = 1
	}
}
