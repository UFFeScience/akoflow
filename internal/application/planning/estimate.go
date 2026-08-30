package planning

import (
	"math"

	"github.com/UFFeScience/akoflow/internal/domain"
)

const (
	// Calibrated against the compact persistent PRISM search. The previous
	// clone-and-rescan implementation was roughly an order of magnitude slower.
	prismSecondsPerExpansion = 0.000009
	// HEFT evaluates placements without cloning the accumulated schedule. This
	// coefficient is calibrated against the 6,448-activity Montage fixture.
	heftSecondsPerEvaluation = 0.00000055
)

func estimateAlgorithmRun(
	algorithm string,
	configuration map[string]any,
	workflow domain.WorkflowVersion,
	scope domain.ExecutionScope,
	resources []domain.Resource,
) domain.PlanningEstimate {
	activityCount := len(workflow.Activities)
	dependencyCount := len(workflow.Dependencies)
	compatible := compatiblePlanningResources(workflow, scope, resources)
	resourceCount := max(1, len(compatible))
	estimate := domain.PlanningEstimate{
		ActivityCount:       activityCount,
		DependencyCount:     dependencyCount,
		CompatibleResources: resourceCount,
		Confidence:          "calibrated",
	}

	if algorithm == "prism-time" || algorithm == "prism-cost" {
		beamWidth := estimateIntOption(configuration, "beamWidth", 120, 1, 10000)
		branchLimit := estimateIntOption(configuration, "readyBranchLimit", 3, 1, 16)
		branchLimit = min(branchLimit, max(1, maximumReadyWidth(workflow)))
		expansions := int64(activityCount) * int64(beamWidth) * int64(branchLimit) * int64(resourceCount)
		estimate.BeamWidth = beamWidth
		estimate.ReadyBranchLimit = branchLimit
		estimate.ExpandedStates = expansions
		estimate.DurationSeconds = roundEstimate(
			0.15 + float64(expansions)*prismSecondsPerExpansion + float64(dependencyCount)*0.000002,
		)
		return estimate
	}

	coreEvaluations := 0
	for _, resource := range compatible {
		cores := resource.CPUCores
		if cores < 1 || opaquePlanningBatchTarget(resource) {
			cores = 1
		}
		coreEvaluations += cores
	}
	work := int64(activityCount * max(1, coreEvaluations))
	estimate.ExpandedStates = work
	estimate.DurationSeconds = roundEstimate(
		0.05 + float64(work)*heftSecondsPerEvaluation + float64(dependencyCount)*0.000001,
	)
	return estimate
}

func opaquePlanningBatchTarget(resource domain.Resource) bool {
	if resource.ExecutionTarget != domain.ExecutionTargetBatch {
		return false
	}
	return resource.Type == domain.ResourceBatchQueue ||
		resource.Type == domain.ResourceHPCPartition ||
		resource.Type == domain.ResourceSlurmReservation
}

func compatiblePlanningResources(
	workflow domain.WorkflowVersion,
	scope domain.ExecutionScope,
	resources []domain.Resource,
) []domain.Resource {
	allowed := map[string]bool{}
	for _, id := range scope.EnvironmentVersionIDs {
		allowed[id] = true
	}
	compatible := make([]domain.Resource, 0, len(resources))
	for _, resource := range resources {
		if !resource.Schedulable || !allowed[resource.EnvironmentVersionID] {
			continue
		}
		supportsWorkflow := true
		if resource.ExecutionTarget != domain.ExecutionTargetBatch {
			for _, activity := range workflow.Activities {
				if resource.CPUCapacity < activity.Resources.CPU || resource.MemoryBytes < activity.Resources.MemoryBytes {
					supportsWorkflow = false
					break
				}
			}
		}
		if supportsWorkflow {
			compatible = append(compatible, resource)
		}
	}
	return compatible
}

func maximumReadyWidth(workflow domain.WorkflowVersion) int {
	indegree := map[string]int{}
	successors := map[string][]string{}
	for _, activity := range workflow.Activities {
		indegree[activity.ID] = 0
	}
	for _, dependency := range workflow.Dependencies {
		indegree[dependency.ActivityID]++
		successors[dependency.DependsOnActivityID] = append(successors[dependency.DependsOnActivityID], dependency.ActivityID)
	}
	ready := make([]string, 0)
	for id, degree := range indegree {
		if degree == 0 {
			ready = append(ready, id)
		}
	}
	maximum := len(ready)
	for len(ready) > 0 {
		current := ready
		ready = nil
		for _, id := range current {
			for _, successor := range successors[id] {
				indegree[successor]--
				if indegree[successor] == 0 {
					ready = append(ready, successor)
				}
			}
		}
		maximum = max(maximum, len(ready))
	}
	return maximum
}

func estimateIntOption(configuration map[string]any, key string, fallback, minimum, maximum int) int {
	value := fallback
	switch raw := configuration[key].(type) {
	case float64:
		value = int(raw)
	case int:
		value = raw
	}
	return max(minimum, min(value, maximum))
}

func roundEstimate(value float64) float64 {
	return math.Round(value*10) / 10
}
