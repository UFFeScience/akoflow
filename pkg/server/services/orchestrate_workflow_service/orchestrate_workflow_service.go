package orchestrate_workflow_service

import (
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

func (o *OrchestrateWorflowService) handleSomeActivitiesRunning(workflow workflow.Workflow) {
	println("handleSomeActivitiesRunning")
}

func (o *OrchestrateWorflowService) handleAllActivitiesFinished(workflow workflow.Workflow) {
	println("handleAllActivitiesFinished")
}

func (o *OrchestrateWorflowService) iterateWorkflows(workflows []workflow.Workflow) {

	// map situation to function
	mapSituationAction := map[string]func(workflows workflow.Workflow){
		SITUATION_ALL_ACTIVITIES_CREATED:  o.handleAllActivitiesCreated,
		SITUATION_SOME_ACTIVITIES_RUNNING: o.handleSomeActivitiesRunning,
	}

	for _, wf := range workflows {
		situation := o.getSituation(wf)
		mapSituationAction[situation](wf)
	}
}

func (d *OrchestrateWorflowService) getSituation(wf workflow.Workflow) string {

	for _, activity := range wf.Spec.Activities {
		if activity.Status == activities_repository.StatusCreated {
			return SITUATION_ALL_ACTIVITIES_CREATED
		}
	}
	return SITUATION_SOME_ACTIVITIES_RUNNING
}

func (d *OrchestrateWorflowService) Orchestrate(workflows []workflow.Workflow) {
	d.iterateWorkflows(workflows)
}
