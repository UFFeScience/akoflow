package algorithms

import (
	"fmt"
	"math"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func addCloudLifecycle(plan *domain.SchedulePlan, request domain.PlanningRequest) {
	resources := make(map[string]domain.Resource, len(request.Resources))
	for _, resource := range request.Resources {
		resources[resource.ID] = resource
	}
	byResource := map[string][]domain.PlanAssignment{}
	for _, assignment := range plan.Assignments {
		resource := resources[assignment.ResourceID]
		if resource.Type == domain.ResourceCloudVM {
			byResource[resource.ID] = append(byResource[resource.ID], assignment)
		}
	}
	for resourceID, assignments := range byResource {
		resource := resources[resourceID]
		first, last, runtimeCost := math.MaxFloat64, 0.0, 0.0
		activityIDs := make([]string, 0, len(assignments))
		for _, assignment := range assignments {
			activityIDs = append(activityIDs, assignment.ID)
			if assignment.PredictedStartAt < first {
				first = assignment.PredictedStartAt
			}
			if assignment.PredictedFinishAt > last {
				last = assignment.PredictedFinishAt
			}
			runtimeCost += assignment.PredictedRuntimeSeconds * resource.PricePerSecond
		}
		provision := metadataSeconds(resource.Metadata, "terraformSeconds", 40)
		sshWait := metadataSeconds(resource.Metadata, "sshWaitSeconds", 20)
		configure := metadataSeconds(resource.Metadata, "configurationSeconds", 30)
		validate := metadataSeconds(resource.Metadata, "validationSeconds", 5)
		stop := metadataSeconds(resource.Metadata, "stopSeconds", metadataSeconds(resource.Metadata, "destroySeconds", 20))
		totalPreparation := provision + sshWait + configure + validate
		start := math.Max(0, first-totalPreparation)
		prefix := plan.ID + "-lifecycle-" + resourceID
		plan.LifecycleActions = append(plan.LifecycleActions,
			domain.PlannedLifecycleAction{ID: prefix + "-provision", SchedulePlanID: plan.ID, CapacityTargetID: resourceID, Action: "provision", EarliestStart: start, ExpectedDuration: provision, Metadata: map[string]any{"phase": "provisioning"}},
			domain.PlannedLifecycleAction{ID: prefix + "-configure", SchedulePlanID: plan.ID, CapacityTargetID: resourceID, Action: "configure", EarliestStart: start + provision + sshWait, ExpectedDuration: configure, DependsOn: []string{prefix + "-provision"}, Metadata: map[string]any{"phase": "configuration", "sshWaitSeconds": sshWait}},
			domain.PlannedLifecycleAction{ID: prefix + "-validate", SchedulePlanID: plan.ID, CapacityTargetID: resourceID, Action: "validate", EarliestStart: start + provision + sshWait + configure, ExpectedDuration: validate, DependsOn: []string{prefix + "-configure"}, Metadata: map[string]any{"phase": "validation"}},
			domain.PlannedLifecycleAction{ID: prefix + "-stop", SchedulePlanID: plan.ID, CapacityTargetID: resourceID, Action: "stop", EarliestStart: last, ExpectedDuration: stop, DependsOn: activityIDs, Metadata: map[string]any{"phase": "stop", "safetyGate": true}},
		)
		activeSeconds := math.Max(0, last+stop-start)
		vmCost := activeSeconds * resource.PricePerSecond
		diskGiB := float64(resource.StorageBytes) / float64(1<<30)
		diskCost := diskGiB * metadataNumber(resource.Metadata, "diskPricePerGiBMonth") * activeSeconds / (730 * 3600)
		plan.Predicted.Cost += math.Max(0, vmCost-runtimeCost) + diskCost
		if plan.Metadata == nil {
			plan.Metadata = map[string]any{}
		}
		plan.Metadata[fmt.Sprintf("cloudCost.%s", resourceID)] = map[string]any{"compute": vmCost, "disk": diskCost, "idle": math.Max(0, vmCost-runtimeCost), "activeSeconds": activeSeconds}
	}
	plan.Predicted.Feasible = (plan.DeadlineSeconds <= 0 || plan.Predicted.MakespanSeconds <= plan.DeadlineSeconds) && (plan.Budget <= 0 || plan.Predicted.Cost <= plan.Budget)
}

// EnrichCloudLifecycle applies the same lifecycle and cost model to manual and
// imported plans as scheduler-produced plans.
func EnrichCloudLifecycle(plan *domain.SchedulePlan, resources []domain.Resource) {
	if len(plan.LifecycleActions) > 0 {
		return
	}
	addCloudLifecycle(plan, domain.PlanningRequest{Resources: resources})
}

func metadataSeconds(values map[string]any, key string, fallback float64) float64 {
	if value := metadataNumber(values, key); value > 0 {
		return value
	}
	return fallback
}

func metadataNumber(values map[string]any, key string) float64 {
	switch value := values[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}
