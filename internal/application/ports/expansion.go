package ports

import (
	"context"
	"github.com/UFFeScience/akoflow/internal/domain"
)

type WorkflowExpansionStore interface {
	SaveExpansion(context.Context, domain.WorkflowExpansion) (*domain.WorkflowExpansion, error)
	FindExpansionByEvent(context.Context, string, string) (*domain.WorkflowExpansion, error)
	ListExpansions(context.Context, string, string) ([]domain.WorkflowExpansion, error)
}
