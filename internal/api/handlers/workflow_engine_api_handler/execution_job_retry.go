package workflow_engine_api_handler

import (
	applicationexecution "github.com/UFFeScience/akoflow/internal/application/execution"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainqueue "github.com/UFFeScience/akoflow/internal/domain/queue"
)

func configureExecutionJobRetries(job *domainqueue.Job, mode domain.ExecutionMode) {
	applicationexecution.ConfigureExecutionJobRetries(job, mode)
}
