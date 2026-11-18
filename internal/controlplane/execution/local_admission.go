package execution

import (
	"context"
	"maps"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
)

const sharedLocalWaitReason = "waiting for shared local CPU or memory capacity"

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

func (s *Supervisor) markPreparing(ctx context.Context, task *domain.TaskExecution, resource domain.Resource, activity domain.Activity, driver domain.RuntimeDriver) (bool, error) {
	previousReason, _ := task.Metadata["queueReason"].(string)
	task.Metadata = maps.Clone(task.Metadata)
	if task.Metadata == nil {
		task.Metadata = make(map[string]any)
	}
	task.Metadata["queueReason"] = "synchronizing predecessor workspaces and preparing runtime"
	capacity := s.config.SharedLocalCapacity
	store, ok := s.executions.(ports.LocalAdmissionStore)
	if ok && capacity.CPU > 0 && capacity.MemoryBytes > 0 && resource.HasLocalAllocation() && driver == domain.RuntimeDriverLocal {
		cpu, memory := resource.LocalActivityBudget(activity.Resources.CPU, activity.Resources.MemoryBytes)
		reserved, err := store.ReserveLocalActivity(ctx, task, ports.LocalActivityCapacity{CPU: cpu, MemoryBytes: memory}, capacity)
		if err != nil || reserved {
			return reserved, err
		}
		if s.config.ReconcileLocalCapacity != nil {
			if err := s.config.ReconcileLocalCapacity(ctx); err != nil {
				return false, err
			}
		}
		task.Metadata["queueReason"] = sharedLocalWaitReason
		if previousReason == task.Metadata["queueReason"] {
			return false, nil
		}
		return false, s.executions.SaveTask(ctx, *task)
	}
	task.Status = domain.TaskPreparing
	return true, s.executions.SaveTask(ctx, *task)
}

func (s *Supervisor) markLocalLaunching(ctx context.Context, runID, activityID string, attempt int, resource domain.Resource, activity domain.Activity) error {
	store, ok := s.executions.(ports.LocalAdmissionStore)
	if ok && s.config.SharedLocalCapacity.CPU > 0 && resource.HasLocalAllocation() {
		return store.MarkLocalActivityLaunching(ctx, runID, activityID, attempt, strings.TrimSpace(activity.Command.Image) != "")
	}
	return nil
}
