package orchestrate_workflow_service

import (
	"github.com/ovvesley/akoflow/pkg/server/engine/channel"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/services/get_workflow_by_status_service"
)

type OrchestrateWorflowService struct {
	namespace           string
	channelManager      *channel.Manager
	getWorkflowByStatus get_workflow_by_status_service.GetWorkflowByStatusService
}

func New() *OrchestrateWorflowService {
	return &OrchestrateWorflowService{
		namespace:           "akoflow",
		channelManager:      channel.GetInstance(),
		getWorkflowByStatus: get_workflow_by_status_service.New(),
	}
}

func (o *OrchestrateWorflowService) dispatchToWorker(activities []workflow_activity_entity.WorkflowActivities) {
	for _, activity := range activities {
		println("Dispatching to worker activity: ", activity.Name, " with id: ", activity.Id)
		o.channelManager.WorfklowChannel <- channel.DataChannel{Namespace: o.namespace, Job: activity, Id: activity.Id}
	}
}

func (o *OrchestrateWorflowService) handleDispatchToWorker(wf workflow_entity.Workflow) []workflow_activity_entity.WorkflowActivities {
	wfsFinished := o.getWorkflowByStatus.GetActivitiesByStatus(wf, activity_repository.StatusFinished)
	wfsRunning := o.getWorkflowByStatus.GetActivitiesByStatus(wf, activity_repository.StatusRunning)
	wfsNotStarted := o.getWorkflowByStatus.GetActivitiesByStatus(wf, activity_repository.StatusCreated)

	println("wfsFinished: ", len(wfsFinished))
	println("wfsRunning: ", len(wfsRunning))
	println("wfsNotStarted: ", len(wfsNotStarted))

	wfNextToRun := o.nextToRun(wfsNotStarted, wfsFinished)

	for _, wfNextToRun := range wfNextToRun {
		o.dispatchToWorker([]workflow_activity_entity.WorkflowActivities{wfNextToRun})
	}

	return wfNextToRun

}

func (o *OrchestrateWorflowService) nextToRun(wfsPending []workflow_activity_entity.WorkflowActivities, wfsFinished []workflow_activity_entity.WorkflowActivities) []workflow_activity_entity.WorkflowActivities {

	wfsNextToRun := make([]workflow_activity_entity.WorkflowActivities, 0)
	for _, wfPending := range wfsPending {
		if o.isDependentOnFinished(wfPending, wfsFinished) {
			if wfPending.Id != 0 {
				wfsNextToRun = append(wfsNextToRun, wfPending)
			}
		}
	}

	return wfsNextToRun
}

func (o *OrchestrateWorflowService) isDependentOnFinished(wfaPending workflow_activity_entity.WorkflowActivities, wfasFinished []workflow_activity_entity.WorkflowActivities) bool {

	mapNameCompleted := make(map[string]bool)

	for _, wfaFinished := range wfasFinished {
		if wfaPending.DependsOn == nil {
			return true
		}

		for _, dependOn := range wfaPending.DependsOn {
			if dependOn == wfaFinished.Name {
				mapNameCompleted[wfaFinished.Name] = true
			}
		}
	}

	if len(wfaPending.DependsOn) == len(mapNameCompleted) {
		return true
	}

	return false
}

func (o *OrchestrateWorflowService) iterateWorkflows(workflows []workflow_entity.Workflow) map[int][]workflow_activity_entity.WorkflowActivities {

	mapWfWfs := make(map[int][]workflow_activity_entity.WorkflowActivities)
	for _, wf := range workflows {
		mapWfWfs[wf.Id] = o.handleDispatchToWorker(wf)
	}

	return mapWfWfs
}

func (d *OrchestrateWorflowService) Orchestrate(workflows []workflow_entity.Workflow) map[int][]workflow_activity_entity.WorkflowActivities {
	return d.iterateWorkflows(workflows)
}
