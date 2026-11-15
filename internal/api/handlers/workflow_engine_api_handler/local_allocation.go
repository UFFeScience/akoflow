package workflow_engine_api_handler

import (
	"context"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func (h *Handler) validateLocalAllocation(ctx context.Context, definition *domain.EnvironmentDefinition) error {
	for index, resource := range definition.Resources {
		enabled, _ := resource.Metadata["localAllocation"].(bool)
		if !enabled {
			continue
		}
		if resource.Type != domain.ResourceLocalMachine {
			return fmt.Errorf("local allocation requires a local machine")
		}
		var connection *domain.EnvironmentConnection
		for i := range definition.Connections {
			if definition.Connections[i].Type == domain.ConnectionLocal {
				connection = &definition.Connections[i]
				break
			}
		}
		if connection == nil || h.connectionTest == nil {
			return fmt.Errorf("a local connection is required to validate capacity")
		}
		health := h.connectionTest(ctx, *connection)
		if !health.Healthy || health.Capacity == nil {
			return fmt.Errorf("local capacity is unavailable; check the connection and try again")
		}
		if err := health.Capacity.Validate(resource.CPUCapacity, resource.MemoryBytes); err != nil {
			return err
		}
		resource.CPUCores = int(resource.CPUCapacity)
		resource.Metadata["detectedCpuCores"] = health.Capacity.MaxCPUs
		resource.Metadata["detectedMemoryBytes"] = health.Capacity.MaxMemoryBytes
		definition.Resources[index] = resource
	}
	return nil
}

func (h *Handler) resolveLocalExecutionAllocation(ctx context.Context, request *ports.ExecutionRequest) error {
	if h.resources == nil {
		return nil
	}
	for index, resource := range request.Resources {
		stored, err := h.resources.FindByID(ctx, resource.ID)
		if err != nil {
			return err
		}
		if stored == nil || !stored.HasLocalAllocation() {
			continue
		}
		request.Resources[index] = *stored
		for _, assignment := range request.Plan.Assignments {
			if assignment.ResourceID != stored.ID {
				continue
			}
			for _, activity := range request.Workflow.Activities {
				if activity.ID != assignment.ActivityID {
					continue
				}
				cpu, memory := stored.LocalActivityBudget(activity.Resources.CPU, activity.Resources.MemoryBytes)
				if cpu > stored.CPUCapacity || memory > stored.MemoryBytes {
					return fmt.Errorf("activity %q exceeds the configured CPU or memory allocation of local environment %q", activity.ID, stored.ID)
				}
			}
		}
	}
	return nil
}
