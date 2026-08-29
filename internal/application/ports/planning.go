package ports

import (
	"context"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// PlannerPlugin is the stable boundary implemented by scheduling plugins.
type PlannerPlugin interface {
	Plan(context.Context, domain.PlanningRequest) (domain.SchedulePlan, error)
}

// PlanSource provides a plan without exposing whether it came from a plugin
// or from an imported definition.
type PlanSource interface {
	Build(context.Context, domain.PlanningRequest) (domain.SchedulePlan, error)
}

type PlanValidator interface {
	Validate(domain.SchedulePlan, domain.WorkflowVersion, []domain.Resource, domain.ExecutionScope, domain.NetworkTopology) error
}

type PlanningStore interface {
	CreateSession(context.Context, domain.PlanningSession, []domain.AlgorithmRun) error
	FindSession(context.Context, string) (*domain.PlanningSession, error)
	ListSessions(context.Context) ([]domain.PlanningSession, error)
	ListAlgorithmRuns(context.Context, string) ([]domain.AlgorithmRun, error)
	SetSessionRunning(context.Context, string) error
	SetSessionCompleted(context.Context, string) error
	SetSessionFailed(context.Context, string, string) error
	SetAlgorithmRunRunning(context.Context, string) error
	UpdateAlgorithmRunProgress(context.Context, string, float64) error
	UpdateSessionProgress(context.Context, string, float64) error
	SetAlgorithmRunCompleted(context.Context, string) error
	SetAlgorithmRunFailed(context.Context, string, string) error
	SaveCandidate(context.Context, domain.PlanCandidate) error
	ListCandidates(context.Context, string) ([]domain.PlanCandidate, error)
	FindCandidate(context.Context, string) (*domain.PlanCandidate, error)
	UpdateCandidateRanks(context.Context, []domain.PlanCandidate) error
	SelectCandidate(context.Context, string, domain.PlanCandidate) error
}

type ProgressReporter interface {
	Report(context.Context, float64, string) error
}

type CandidateSink interface {
	Emit(context.Context, domain.SchedulePlan) error
}

type SchedulerDescriptor struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Objective   string         `json:"objective"`
	Multiple    bool           `json:"multiple"`
	Description string         `json:"description"`
	Defaults    map[string]any `json:"defaults,omitempty"`
}

type Scheduler interface {
	Descriptor() SchedulerDescriptor
	Schedule(context.Context, domain.PlanningRequest, map[string]any, ProgressReporter, CandidateSink) error
}
