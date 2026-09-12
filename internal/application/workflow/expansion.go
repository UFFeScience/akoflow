package workflow

import (
	"context"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainworkflow "github.com/UFFeScience/akoflow/internal/domain/workflow"
)

type ExpansionCoordinator struct {
	Workflows ports.WorkflowStore
	Store     ports.WorkflowExpansionStore
	Limits    domain.ExpansionLimits
}

func (c ExpansionCoordinator) Apply(ctx context.Context, request domain.ExpansionRequest) (*domain.WorkflowExpansion, error) {
	if c.Workflows == nil || c.Store == nil {
		return nil, fmt.Errorf("workflow and expansion stores are required")
	}
	if existing, err := c.Store.FindExpansionByEvent(ctx, request.WorkflowVersionID, request.SourceEventID); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}
	base, err := c.Workflows.FindVersion(ctx, request.WorkflowVersionID)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return nil, fmt.Errorf("workflow version %q was not found", request.WorkflowVersionID)
	}
	prior, err := c.Store.ListExpansions(ctx, request.WorkflowVersionID, request.ExecutionRunID)
	if err != nil {
		return nil, err
	}
	if request.Sequence != len(prior)+1 {
		return c.reject(ctx, request, len(prior)+1, fmt.Errorf("expansion sequence must be %d", len(prior)+1))
	}
	expansion, _, err := domainworkflow.MaterializeExpansion(*base, prior, request, c.Limits)
	if err != nil {
		return c.reject(ctx, request, len(prior)+1, err)
	}
	return c.Store.SaveExpansion(ctx, expansion)
}

func (c ExpansionCoordinator) reject(ctx context.Context, request domain.ExpansionRequest, revision int, cause error) (*domain.WorkflowExpansion, error) {
	value := domain.WorkflowExpansion{
		ID:                domainworkflow.DeterministicExpansionID(request.WorkflowVersionID, request.SourceEventID),
		WorkflowVersionID: request.WorkflowVersionID, ExecutionRunID: request.ExecutionRunID,
		SourceActivityID: request.SourceActivityID, SourceEventID: request.SourceEventID,
		Sequence: request.Sequence, ResultRevision: revision,
		Status: domainworkflow.ExpansionStatusRejected, FailureReason: cause.Error(), Metadata: request.Metadata,
	}
	return c.Store.SaveExpansion(ctx, value)
}

func (c ExpansionCoordinator) Result(ctx context.Context, workflowVersionID, executionRunID string) (*domain.ExpandedWorkflow, error) {
	base, err := c.Workflows.FindVersion(ctx, workflowVersionID)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return nil, fmt.Errorf("workflow version %q was not found", workflowVersionID)
	}
	expansions, err := c.Store.ListExpansions(ctx, workflowVersionID, executionRunID)
	if err != nil {
		return nil, err
	}
	result := &domain.ExpandedWorkflow{
		WorkflowVersionID: workflowVersionID,
		Activities:        append([]domain.Activity{}, base.Activities...),
		Dependencies:      append([]domain.ActivityDependency{}, base.Dependencies...),
		Expansions:        expansions,
	}
	for _, expansion := range expansions {
		if expansion.Status != domainworkflow.ExpansionStatusApplied {
			continue
		}
		result.Revision++
		result.Activities = append(result.Activities, expansion.Activities...)
		result.Dependencies = append(result.Dependencies, expansion.Dependencies...)
	}
	return result, nil
}
