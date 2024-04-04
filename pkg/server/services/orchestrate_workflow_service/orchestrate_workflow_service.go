package orchestrate_workflow_service

import (
	"github.com/ovvesley/scik8sflow/pkg/server/channel"
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow"
	"github.com/ovvesley/scik8sflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/scik8sflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/scik8sflow/pkg/server/services/get_workflow_by_status_service"
)

type OrchestrateWorflowService struct {
	namespace           string
	workflowRepository  workflow_repository.IWorkflowRepository
	activityRepository  activity_repository.IActivityRepository
	channelManager      *channel.Manager
	getWorkflowByStatus *get_workflow_by_status_service.GetWorkflowByStatusService
}

func New() *OrchestrateWorflowService {
	return &OrchestrateWorflowService{
		namespace:           "scik8sflow",
		workflowRepository:  workflow_repository.New(),
		activityRepository:  activity_repository.New(),
		channelManager:      channel.GetInstance(),
		getWorkflowByStatus: get_workflow_by_status_service.New(),
	}
}

func (o *OrchestrateWorflowService) dispatchToWorker(activities []workflow.WorkflowActivities) {
	for _, activity := range activities {
		println("Dispatching to worker activity: ", activity.Name, " with id: ", activity.Id)
		o.channelManager.WorfklowChannel <- channel.DataChannel{Namespace: o.namespace, Job: activity, Id: activity.Id}
	}
}

func (o *OrchestrateWorflowService) handleDispatchToWorker(wf workflow.Workflow) {
	wfsFinished := o.getWorkflowByStatus.GetActivitiesByStatus(wf, activity_repository.StatusFinished)
	wfsRunning := o.getWorkflowByStatus.GetActivitiesByStatus(wf, activity_repository.StatusRunning)
	wfsNotStarted := o.getWorkflowByStatus.GetActivitiesByStatus(wf, activity_repository.StatusCreated)

	println("wfsFinished: ", len(wfsFinished))
	println("wfsRunning: ", len(wfsRunning))
	println("wfsNotStarted: ", len(wfsNotStarted))

	wfNextToRun, err := o.nextToRun(wfsNotStarted, wfsFinished)

	if err != nil {
		return
	}

	for _, wfNextToRun := range wfNextToRun {
		o.dispatchToWorker([]workflow.WorkflowActivities{wfNextToRun})
	}

}

func (o *OrchestrateWorflowService) nextToRun(wfsPending []workflow.WorkflowActivities, wfsFinished []workflow.WorkflowActivities) ([]workflow.WorkflowActivities, error) {

	wfsNextToRun := make([]workflow.WorkflowActivities, 0)
	for _, wfPending := range wfsPending {
		if o.isDependentOnFinished(wfPending, wfsFinished) {
			if wfPending.Id != 0 {
				wfsNextToRun = append(wfsNextToRun, wfPending)
			}
		}
	}

	return wfsNextToRun, nil
}

func (o *OrchestrateWorflowService) isDependentOnFinished(wfaPending workflow.WorkflowActivities, wfasFinished []workflow.WorkflowActivities) bool {

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

func (o *OrchestrateWorflowService) iterateWorkflows(workflows []workflow.Workflow) {

	for _, wf := range workflows {
		o.handleDispatchToWorker(wf)
	}
}

func (d *OrchestrateWorflowService) Orchestrate(workflows []workflow.Workflow) {
	d.iterateWorkflows(workflows)
}
