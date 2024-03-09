package orchestrate_workflow_service

import (
	"errors"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/channel"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/activities_repository"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/repository/workflow_repository"
)

// Situação 1: Nenhuma atividade está rodando
var SITUATION_ALL_ACTIVITIES_CREATED = "SITUATION_ALL_ACTIVITIES_CREATED"

// Situação 2: Alguma atividade está rodando
var SITUATION_SOME_ACTIVITIES_RUNNING = "SITUATION_SOME_ACTIVITIES_RUNNING"

// Situação 3: Todas as atividades estão finalizadas
var SITUATION_ALL_ACTIVITIES_FINISHED = "SITUATION_ALL_ACTIVITIES_FINISHED"

type OrchestrateWorflowService struct {
	namespace          string
	workflowRepository *workflow_repository.WorkflowRepository
	activityRepository *activities_repository.ActivityRepository
	channelManager     *channel.Manager
}

func New() *OrchestrateWorflowService {
	return &OrchestrateWorflowService{
		namespace:          "k8science-cluster-manager",
		workflowRepository: workflow_repository.New(),
		activityRepository: activities_repository.New(),
		channelManager:     channel.GetInstance(),
	}
}

func (o *OrchestrateWorflowService) handleAllActivitiesCreated(workflow workflow.Workflow) {
	notDependentActivities := o.getNotDependentActivities(workflow)
	o.dispatchToWorker(notDependentActivities)
}

func (o *OrchestrateWorflowService) dispatchToWorker(activities []workflow.WorkflowActivities) {
	for _, activity := range activities {
		println("Dispatching to worker activity: ", activity.Name, " with id: ", activity.ID)
		o.channelManager.WorfklowChannel <- channel.DataChannel{Namespace: o.namespace, Job: activity, Id: activity.ID}
	}
}

func (o *OrchestrateWorflowService) getNotDependentActivities(wf workflow.Workflow) []workflow.WorkflowActivities {
	var notDependentActivities []workflow.WorkflowActivities
	for _, activity := range wf.Spec.Activities {
		if activity.DependOnActivity == nil {
			notDependentActivities = append(notDependentActivities, activity)
		}
	}
	return notDependentActivities
}

func (o *OrchestrateWorflowService) handleSomeActivitiesRunningOrFinished(wf workflow.Workflow) {
	wfsFinished := o.getWorkflowsByStatus(wf, activities_repository.StatusFinished)
	wfsRunning := o.getWorkflowsByStatus(wf, activities_repository.StatusRunning)
	wfsNotStarted := o.getWorkflowsByStatus(wf, activities_repository.StatusCreated)

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

	if wfNextToRun.ID == 0 {
		return wfNextToRun, errors.New("No activity to run")
	}

	return wfNextToRun, nil
}

func (o *OrchestrateWorflowService) isDependentOnFinished(wfaPending workflow.WorkflowActivities, wfasFinished []workflow.WorkflowActivities) bool {
	for _, wfaFinished := range wfasFinished {
		if wfaPending.DependOnActivity == nil {
			return true
		}

		if *wfaPending.DependOnActivity == wfaFinished.ID {
			return true
		}
	}
	return false
}

func (o *OrchestrateWorflowService) getWorkflowsByStatus(wfs workflow.Workflow, status int) []workflow.WorkflowActivities {
	var wfsSelected []workflow.WorkflowActivities
	for _, activity := range wfs.Spec.Activities {
		if activity.Status == status {
			wfsSelected = append(wfsSelected, activity)
		}
	}
	return wfsSelected
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
