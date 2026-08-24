package planning

import (
	"context"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type Plugin = ports.PlannerPlugin
type Source = ports.PlanSource

type PluginSource struct{ Plugin Plugin }

func (s PluginSource) Build(ctx context.Context, request domain.PlanningRequest) (domain.SchedulePlan, error) {
	plan, err := s.Plugin.Plan(ctx, request)
	if err == nil {
		plan.Source = domain.PlanningSourcePlugin
	}
	return plan, err
}

type ImportedSource struct{ Plan domain.SchedulePlan }

func (s ImportedSource) Build(_ context.Context, _ domain.PlanningRequest) (domain.SchedulePlan, error) {
	plan := s.Plan
	plan.Source = domain.PlanningSourceImported
	return plan, nil
}
