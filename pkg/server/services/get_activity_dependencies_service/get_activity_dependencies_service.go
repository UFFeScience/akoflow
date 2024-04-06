package get_activity_dependencies_service

import (
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow"
	"github.com/ovvesley/scik8sflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/scik8sflow/pkg/server/repository/workflow_repository"
)

// GetActivityDependenciesService is a service that returns the dependencies of an activity.
type GetActivityDependenciesService struct {
	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository
}

func New() GetActivityDependenciesService {
	return GetActivityDependenciesService{
		workflowRepository: workflow_repository.New(),
		activityRepository: activity_repository.New(),
	}
}

type ActivityDependencies map[int][]workflow.WorkflowActivities

func (g *GetActivityDependenciesService) GetActivityDependencies(workflowId int) ActivityDependencies {
	wf, _ := g.workflowRepository.Find(workflowId)
	wfa, _ := g.activityRepository.GetActivitiesByWorkflowIds([]int{workflowId})
	wfaDependencies, _ := g.activityRepository.GetWfaDependencies(workflowId)
	activityDependencies := make(ActivityDependencies)
	setDependencies := make(map[int]map[int]workflow.WorkflowActivities)

	mapWfa := make(map[int]workflow.WorkflowActivities)
	for _, w := range wfa[wf.Id] {
		mapWfa[w.Id] = w
		activityDependencies[w.Id] = make([]workflow.WorkflowActivities, 0)
		setDependencies[w.Id] = make(map[int]workflow.WorkflowActivities)
	}

	for _, wfaDep := range wfaDependencies {
		for _, dep := range g.fillActivityDependencies(wfaDep.DependsOnId, mapWfa, wfaDependencies) {
			setDependencies[wfaDep.ActivityId][dep.Id] = dep
		}
		activityDependencies[wfaDep.ActivityId] = g.setDependenciesToArray(setDependencies[wfaDep.ActivityId])
	}

	return activityDependencies
}

// fillActivityDependencies is a recursive function that fills the dependencies of an activity and its dependencies. Critical to the GetActivityDependencies function.
func (g *GetActivityDependenciesService) fillActivityDependencies(dependWfa int, mapWfa map[int]workflow.WorkflowActivities, wfaDependencies []workflow.WorkflowActivityDependencyDatabase) []workflow.WorkflowActivities {
	setDependencies := make(map[int]workflow.WorkflowActivities)

	if wfa, ok := mapWfa[dependWfa]; ok {
		setDependencies[wfa.Id] = wfa
		for _, wfaDep := range wfaDependencies {
			if wfaDep.ActivityId == dependWfa {
				for _, dep := range g.fillActivityDependencies(wfaDep.DependsOnId, mapWfa, wfaDependencies) {
					setDependencies[dep.Id] = dep
				}
			}
		}
	}
	return g.setDependenciesToArray(setDependencies)
}

func (g *GetActivityDependenciesService) setDependenciesToArray(setDependencies map[int]workflow.WorkflowActivities) []workflow.WorkflowActivities {
	dependencies := make([]workflow.WorkflowActivities, 0)
	for _, dep := range setDependencies {
		dependencies = append(dependencies, dep)
	}

	sorted := make([]workflow.WorkflowActivities, 0)

	for i := 0; i < len(dependencies); i++ {
		for j := i + 1; j < len(dependencies); j++ {
			if dependencies[i].Id > dependencies[j].Id {
				dependencies[i], dependencies[j] = dependencies[j], dependencies[i]
			}
		}
		sorted = append(sorted, dependencies[i])

	}
	return sorted

}
