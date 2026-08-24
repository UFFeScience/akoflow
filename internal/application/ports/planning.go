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
