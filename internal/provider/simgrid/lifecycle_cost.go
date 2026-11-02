package simgrid

import (
	"math"

	"github.com/UFFeScience/akoflow/internal/domain"
)

// Lifecycle billing is separate from simulated CPU work. Charge each cloud
// window once, including provisioning/idle/stop and provisioned disk; never
// overwrite an observed trace with the plan's predicted total cost.
func applyCloudLifecycleCosts(tasks []domain.TaskExecution, resources []domain.Resource, actions []domain.PlannedLifecycleAction) {
	for _, resource := range resources {
		if resource.Type != domain.ResourceCloudVM {
			continue
		}
		start, stopDuration, found := math.MaxFloat64, 0.0, false
		for _, action := range actions {
			if action.CapacityTargetID != resource.ID {
				continue
			}
			if action.Action == "provision" {
				start = math.Min(start, action.EarliestStart)
				found = true
			}
			if action.Action == "stop" {
				stopDuration = math.Max(stopDuration, action.ExpectedDuration)
			}
		}
		if !found {
			continue
		}
		indexes := []int{}
		finish, runtime := 0.0, 0.0
		for i, task := range tasks {
			if task.AllocatedResourceID != resource.ID {
				continue
			}
			indexes = append(indexes, i)
			finish = math.Max(finish, task.FinishedAt)
			start = math.Min(start, task.StartedAt)
			runtime += task.RuntimeSeconds
		}
		if len(indexes) == 0 {
			continue
		}
		seconds := math.Max(0, finish+stopDuration-start)
		diskPrice, _ := resource.Metadata["diskPricePerGiBMonth"].(float64)
		disk := float64(resource.StorageBytes) / float64(1<<30) * diskPrice * seconds / (730 * 3600)
		total := seconds*resource.PricePerSecond + disk
		for _, i := range indexes {
			share := 1.0 / float64(len(indexes))
			if runtime > 0 {
				share = tasks[i].RuntimeSeconds / runtime
			}
			tasks[i].Cost = total * share
			if tasks[i].Metadata == nil {
				tasks[i].Metadata = map[string]any{}
			}
			tasks[i].Metadata["costBasis"] = "cloud-lifecycle-compute-and-disk"
			tasks[i].Metadata["cloudBillingWindowSeconds"] = seconds
			tasks[i].Metadata["cloudDiskCostShare"] = disk * share
		}
	}
}
