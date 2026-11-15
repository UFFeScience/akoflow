package execution

import "github.com/UFFeScience/akoflow/internal/domain"

type localReservation struct {
	cpu    float64
	memory int64
}

func localCapacityReady(
	ready []string,
	assignments map[string]domain.PlanAssignment,
	resources map[string]domain.Resource,
	activities map[string]domain.Activity,
	tasks map[string]domain.TaskExecution,
	pending map[string]bool,
) []string {
	reserved := map[string]localReservation{}
	for id, task := range tasks {
		if task.Status != domain.TaskPreparing && task.Status != domain.TaskRunning && !pending[id] {
			continue
		}
		reserveLocalActivity(id, assignments, resources, activities, reserved)
	}
	eligible := []string{}
	for _, id := range ready {
		resource := resources[assignments[id].ResourceID]
		if resource.HasLocalAllocation() {
			activity := activities[id]
			cpu, memory := resource.LocalActivityBudget(activity.Resources.CPU, activity.Resources.MemoryBytes)
			used := reserved[resource.ID]
			if used.cpu+cpu > resource.CPUCapacity || used.memory+memory > resource.MemoryBytes {
				continue
			}
			reserveLocalActivity(id, assignments, resources, activities, reserved)
		}
		eligible = append(eligible, id)
	}
	return eligible
}

func reserveLocalActivity(id string, assignments map[string]domain.PlanAssignment, resources map[string]domain.Resource, activities map[string]domain.Activity, reserved map[string]localReservation) {
	resource := resources[assignments[id].ResourceID]
	if !resource.HasLocalAllocation() {
		return
	}
	activity := activities[id]
	cpu, memory := resource.LocalActivityBudget(activity.Resources.CPU, activity.Resources.MemoryBytes)
	used := reserved[resource.ID]
	reserved[resource.ID] = localReservation{cpu: used.cpu + cpu, memory: used.memory + memory}
}

func capacityReadyActivities(
	ready []string,
	assignments map[string]domain.PlanAssignment,
	resources map[string]domain.Resource,
	activities map[string]domain.Activity,
	tasks map[string]domain.TaskExecution,
	pending map[string]bool,
) []string {
	ready = restrictedCapacityReady(ready, assignments, resources, tasks, pending)
	return localCapacityReady(ready, assignments, resources, activities, tasks, pending)
}
