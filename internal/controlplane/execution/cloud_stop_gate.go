package execution

import (
	"context"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

func (s *Supervisor) stopIdleCloud(
	ctx context.Context,
	request ports.ExecutionRequest,
	completed map[string]domain.TaskExecution,
	tasks map[string]domain.TaskExecution,
	stopping map[string]bool,
) {
	stopper, ok := s.config.CloudAllocator.(CloudStopper)
	if !ok {
		return
	}
	ready := readyCloudStops(
		request, completed, tasks, stopping,
		s.config.Data != nil && s.config.Preparer != nil,
	)
	for instanceID, allocations := range ready {
		if err := stopper.Stop(ctx, request.Run.ID, allocations); err != nil {
			// Teardown must not change the scientific run result. The final
			// release path will retry if infrastructure submission failed.
			continue
		}
		stopping[instanceID] = true
	}
}

func plannedStopResources(plan domain.SchedulePlan, resources map[string]domain.Resource) map[string]bool {
	stopTargets := make(map[string]bool)
	for _, action := range plan.LifecycleActions {
		if action.Action == "stop" {
			stopTargets[action.CapacityTargetID] = true
		}
	}
	selected := make(map[string]bool)
	for resourceID, resource := range resources {
		targetID := resourceCapacityTarget(resource)
		selected[resourceID] = stopTargets[resourceID] || stopTargets[targetID]
	}
	return selected
}

func resourceCapacityTarget(resource domain.Resource) string {
	if value, ok := resource.Metadata["capacityTargetId"].(string); ok && value != "" {
		return value
	}
	return resource.ID
}

// A VM can stop only after its work has finished and every downstream activity
// on another resource has passed its preparation gate (and therefore copied
// the producer workspace). Running the consumer is sufficient; its own work
// need not finish before the source VM stops.
func readyCloudStops(
	request ports.ExecutionRequest,
	completed map[string]domain.TaskExecution,
	tasks map[string]domain.TaskExecution,
	stopping map[string]bool,
	transferGateAvailable bool,
) map[string]map[string]domain.RuntimeAllocation {
	assignments := indexAssignments(request.Plan.Assignments)
	resources := indexResources(request.Resources)
	stopResources := plannedStopResources(request.Plan, resources)
	byInstance := make(map[string]map[string]domain.RuntimeAllocation)
	for activityID, allocation := range request.RuntimeAllocations {
		if allocation.CloudInstanceID == "" || !stopResources[assignments[activityID].ResourceID] {
			continue
		}
		if byInstance[allocation.CloudInstanceID] == nil {
			byInstance[allocation.CloudInstanceID] = make(map[string]domain.RuntimeAllocation)
		}
		byInstance[allocation.CloudInstanceID][activityID] = allocation
	}
	ready := make(map[string]map[string]domain.RuntimeAllocation)
	for instanceID, allocations := range byInstance {
		if stopping[instanceID] || !cloudWorkFinished(request, assignments, allocations, completed) {
			continue
		}
		if !cloudOutputsTransferred(request.Workflow, assignments, allocations, tasks, transferGateAvailable) {
			continue
		}
		ready[instanceID] = allocations
	}
	return ready
}

func cloudWorkFinished(
	request ports.ExecutionRequest,
	assignments map[string]domain.PlanAssignment,
	allocations map[string]domain.RuntimeAllocation,
	completed map[string]domain.TaskExecution,
) bool {
	resources := indexResources(request.Resources)
	targetIDs := make(map[string]bool)
	for activityID := range allocations {
		resource := resources[assignments[activityID].ResourceID]
		targetIDs[resourceCapacityTarget(resource)] = true
	}
	for _, assignment := range request.Plan.Assignments {
		if targetIDs[resourceCapacityTarget(resources[assignment.ResourceID])] {
			if _, done := completed[assignment.ActivityID]; !done {
				return false
			}
		}
	}
	return true
}

func cloudOutputsTransferred(
	workflow domain.WorkflowVersion,
	assignments map[string]domain.PlanAssignment,
	allocations map[string]domain.RuntimeAllocation,
	tasks map[string]domain.TaskExecution,
	transferGateAvailable bool,
) bool {
	for _, consumer := range workflow.Activities {
		for _, producerID := range workspaceProducers(workflow, consumer.ID) {
			if _, producedHere := allocations[producerID]; !producedHere {
				continue
			}
			if assignments[producerID].ResourceID == assignments[consumer.ID].ResourceID {
				continue
			}
			if !transferGateAvailable {
				return false
			}
			status := tasks[consumer.ID].Status
			if status != domain.TaskRunning && status != domain.TaskCompleted {
				return false
			}
		}
	}
	return true
}
