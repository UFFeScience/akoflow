package orchestrate_workflow_service

import (
	"errors"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/channel"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/activities_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/get_workflow_by_status_service"
)

// Situação 1: Nenhuma atividade está rodando
var SITUATION_ALL_ACTIVITIES_CREATED = "SITUATION_ALL_ACTIVITIES_CREATED"

// Situação 2: Alguma atividade está rodando
var SITUATION_SOME_ACTIVITIES_RUNNING = "SITUATION_SOME_ACTIVITIES_RUNNING"

// Situação 3: Todas as atividades estão finalizadas
var SITUATION_ALL_ACTIVITIES_FINISHED = "SITUATION_ALL_ACTIVITIES_FINISHED"

type OrchestrateWorflowService struct {
	namespace           string
	workflowRepository  workflow_repository.IWorkflowRepository
	activityRepository  activities_repository.IActivityRepository
	channelManager      *channel.Manager
	getWorkflowByStatus *get_workflow_by_status_service.GetWorkflowByStatusService
}

func New() *OrchestrateWorflowService {
	return &OrchestrateWorflowService{
		namespace:           "k8science-cluster-manager",
		workflowRepository:  workflow_repository.New(),
		activityRepository:  activities_repository.New(),
		channelManager:      channel.GetInstance(),
		getWorkflowByStatus: get_workflow_by_status_service.New(),
	}
}

func (o *OrchestrateWorflowService) handleAllActivitiesCreated(workflow workflow.Workflow) {
	notDependentActivities := o.getNotDependentActivities(workflow)
	o.dispatchToWorker(notDependentActivities)
}

func (o *OrchestrateWorflowService) dispatchToWorker(activities []workflow.WorkflowActivities) {
	for _, activity := range activities {
		println("Dispatching to worker activity: ", activity.Name, " with id: ", activity.Id)
		o.channelManager.WorfklowChannel <- channel.DataChannel{Namespace: o.namespace, Job: activity, Id: activity.Id}
	}
}

func (o *OrchestrateWorflowService) getNotDependentActivities(wf workflow.Workflow) []workflow.WorkflowActivities {
	var notDependentActivities []workflow.WorkflowActivities
	for _, activity := range wf.Spec.Activities {
		if len(activity.DependsOn) == 0 {
			notDependentActivities = append(notDependentActivities, activity)
		}
	}
	return notDependentActivities
}

func (o *OrchestrateWorflowService) handleSomeActivitiesRunningOrFinished(wf workflow.Workflow) {
	wfsFinished := o.getWorkflowByStatus.GetActivitiesByStatus(wf, activities_repository.StatusFinished)
	wfsRunning := o.getWorkflowByStatus.GetActivitiesByStatus(wf, activities_repository.StatusRunning)
	wfsNotStarted := o.getWorkflowByStatus.GetActivitiesByStatus(wf, activities_repository.StatusCreated)

	println("wfsFinished: ", len(wfsFinished))
	println("wfsRunning: ", len(wfsRunning))
	println("wfsNotStarted: ", len(wfsNotStarted))

	wfNextToRun, err := o.nextToRun(wfsNotStarted, wfsFinished)

	if err != nil {
		return
	}

	o.dispatchToWorker([]workflow.WorkflowActivities{wfNextToRun})

}

func (o *OrchestrateWorflowService) nextToRun(wfsPending []workflow.WorkflowActivities, wfsFinished []workflow.WorkflowActivities) (workflow.WorkflowActivities, error) {
	var wfNextToRun workflow.WorkflowActivities
	for _, wfPending := range wfsPending {
		if o.isDependentOnFinished(wfPending, wfsFinished) {
			wfNextToRun = wfPending
		}
	}

	if wfNextToRun.Id == 0 {
		return wfNextToRun, errors.New("No activity to run")
	}

	return wfNextToRun, nil
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

func (o *OrchestrateWorflowService) getMapSituationAction() map[string]func(workflows workflow.Workflow) {
	mapSituationAction := map[string]func(workflows workflow.Workflow){
		SITUATION_ALL_ACTIVITIES_CREATED:  o.handleAllActivitiesCreated,
		SITUATION_SOME_ACTIVITIES_RUNNING: o.handleSomeActivitiesRunningOrFinished,
		SITUATION_ALL_ACTIVITIES_FINISHED: o.handleSomeActivitiesRunningOrFinished,
	}
	return mapSituationAction
}

func (o *OrchestrateWorflowService) iterateWorkflows(workflows []workflow.Workflow) {

	for _, wf := range workflows {
		situation := o.getSituation(wf)
		o.getMapSituationAction()[situation](wf)
	}
}

func (d *OrchestrateWorflowService) getSituation(wf workflow.Workflow) string {

	for _, activity := range wf.Spec.Activities {
		if activity.Status == activities_repository.StatusRunning {
			return SITUATION_SOME_ACTIVITIES_RUNNING
		}
		if activity.Status == activities_repository.StatusFinished {
			return SITUATION_ALL_ACTIVITIES_FINISHED
		}
	}
	return SITUATION_ALL_ACTIVITIES_CREATED
}

func (d *OrchestrateWorflowService) Orchestrate(workflows []workflow.Workflow) {
	d.iterateWorkflows(workflows)
}
