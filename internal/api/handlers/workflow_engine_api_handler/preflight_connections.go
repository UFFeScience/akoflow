package workflow_engine_api_handler

import (
	"context"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	apppreflight "github.com/UFFeScience/akoflow/internal/application/preflight"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func (h *Handler) preflightConnections(request *ports.ExecutionRequest) []apppreflight.Probe {
	probes := []apppreflight.Probe{}
	assigned := map[string]bool{}
	for _, a := range request.Plan.Assignments {
		assigned[a.ResourceID] = true
	}
	for index := range request.Resources {
		resource := request.Resources[index]
		if !assigned[resource.ID] {
			continue
		}
		probes = append(probes, apppreflight.Probe{Code: "environment_connection", Stage: "connections", EntityID: resource.ID, Path: "/environments", Run: func(ctx context.Context) (string, string, string) {
			resource := request.Resources[index]
			id, _ := resource.Metadata["connectionId"].(string)
			connection := &domain.EnvironmentConnection{Type: domain.ConnectionLocal}
			if id != "" {
				store, ok := h.environments.(ports.ConnectionStore)
				if !ok {
					return "unverified", "Connection credentials cannot be resolved by this installation.", "Review the destination connection."
				}
				value, err := store.FindConnection(ctx, id)
				if err != nil {
					return "blocked", err.Error(), "Restore access to connection configuration."
				}
				if value == nil {
					return "blocked", "The destination connection reference does not exist.", "Register a valid destination connection."
				}
				connection = value
			} else if resource.Type != domain.ResourceLocalMachine {
				return "unverified", "No probeable connection is associated with this destination; provisioned resources are not created by validation.", "Review the resource connection and provisioning requirements."
			}
			if h.connectionTest == nil {
				return "unverified", "Live connectivity checks are unavailable.", "Check the destination connection before execution."
			}
			health := h.connectionTest(ctx, *connection)
			if !health.Healthy {
				return "blocked", "Connection or credential validation failed: " + health.Message, "Correct credentials or restore connectivity in the environment settings."
			}
			return "passed", "Connection access was verified with the configured credential references.", ""
		}})
	}
	return probes
}
