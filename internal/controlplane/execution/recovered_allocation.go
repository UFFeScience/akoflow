package execution

import "github.com/UFFeScience/akoflow/internal/domain"

// Reused output remains on the exact instance recorded by its completed
// attempt. Never substitute a ready instance with the same capacity target.
func recoveredAllocation(task domain.TaskExecution) domain.RuntimeAllocation {
	resourceID := task.AllocatedResourceID
	if resourceID == "" {
		resourceID = task.PlannedResourceID
	}
	return domain.RuntimeAllocation{
		ResourceID: resourceID, RuntimeID: task.RuntimeID,
		ConnectionID: task.ConnectionID, CloudInstanceID: task.CloudInstanceID,
	}
}
