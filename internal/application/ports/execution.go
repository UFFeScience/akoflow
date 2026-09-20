package ports

import (
	"context"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type ExecutionRequest struct {
	Run                               domain.ExecutionRun
	Plan                              domain.SchedulePlan
	Workflow                          domain.WorkflowVersion
	Resources                         []domain.Resource
	ExecutionScope                    domain.ExecutionScope
	Runtimes                          []domain.EnvironmentRuntime
	RuntimeBindings                   []domain.ResourceRuntimeBinding
	NetworkTopology                   domain.NetworkTopology
	ActivityProfiles                  []domain.ActivityResourceProfile
	PreparationRequirementsByActivity map[string]domain.PreparationRequirement
	RuntimeAllocations                map[string]domain.RuntimeAllocation
}

type PreparationCoordinator interface {
	Prepare(context.Context, string, domain.PreparationRequirement) (*domain.PreparationGate, error)
}

type PlanExecutor interface {
	Execute(context.Context, ExecutionRequest) (domain.ExecutionTrace, error)
}

type RuntimeAdapter interface {
	Modes() []domain.ExecutionMode
	Start(context.Context, domain.ActivityExecutionContext) (domain.ActivityHandle, error)
	Inspect(context.Context, domain.ActivityHandle) (domain.ActivityHandle, error)
	Stop(context.Context, domain.ActivityHandle) error
}

// RuntimeResolver selects an adapter from execution mode and runtime identity.
// It is the only point where application code knows that multiple execution
// technologies exist.
type RuntimeResolver interface {
	Resolve(domain.ExecutionMode, string) (RuntimeAdapter, error)
}

type ActivityExecutionStore interface {
	Save(context.Context, domain.ActivityHandle) error
	Find(context.Context, string) (*domain.ActivityHandle, error)
}

type ExecutionStore interface {
	ActivityExecutionStore
	CreateRun(context.Context, domain.ExecutionRun) error
	FindRun(context.Context, string) (*domain.ExecutionRun, error)
	SaveTask(context.Context, domain.TaskExecution) error
	CompleteRun(context.Context, domain.ExecutionTrace) error
	FailRun(context.Context, string, string) error
}

type WorkspaceStore interface {
	SaveWorkspace(context.Context, domain.ActivityWorkspace) error
	FindWorkspace(context.Context, string) (*domain.ActivityWorkspace, error)
	ListWorkspaces(context.Context, string) ([]domain.ActivityWorkspace, error)
	ListWorkspaceCandidates(context.Context) ([]domain.ActivityWorkspace, error)
	SaveWorkspaceLease(context.Context, domain.WorkspaceLease) error
	ListWorkspaceLeases(context.Context, string) ([]domain.WorkspaceLease, error)
}
