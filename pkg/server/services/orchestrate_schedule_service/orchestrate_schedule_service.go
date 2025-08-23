package orchestrate_schedule_service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"plugin"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/node_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/schedule_repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/services/node_current_metrics_service"
)

type OrchestrateScheduleService struct {
	scheduleRepository schedule_repository.IScheduleRepository
	nodeRepository     node_repository.INodeRepository
	activityRepository activity_repository.IActivityRepository

	nodeCurrentMetricsService node_current_metrics_service.NodeCurrentMetricsService

	workflow             workflow_entity.Workflow
	readyToRunActivities []workflow_activity_entity.WorkflowActivities
}

func New() OrchestrateScheduleService {
	return OrchestrateScheduleService{
		scheduleRepository: config.App().Repository.ScheduleRepository,
		activityRepository: config.App().Repository.ActivityRepository,
		nodeRepository:     config.App().Repository.NodeRepository,

		nodeCurrentMetricsService: node_current_metrics_service.New(),

		workflow:             workflow_entity.Workflow{},
		readyToRunActivities: []workflow_activity_entity.WorkflowActivities{},
	}
}

func (r *OrchestrateScheduleService) SetWorkflow(workflow workflow_entity.Workflow) *OrchestrateScheduleService {
	r.workflow = workflow
	return r
}

func (r *OrchestrateScheduleService) SetReadyToRunActivities(activities []workflow_activity_entity.WorkflowActivities) *OrchestrateScheduleService {
	r.readyToRunActivities = activities
	return r
}

type ResponseStartSchedule map[string]any

func (o *OrchestrateScheduleService) Orchestrate() ([]workflow_activity_entity.WorkflowActivities, error) {

	scheduleName := o.workflow.Spec.Schedule

	newReadyToRunActivities := []workflow_activity_entity.WorkflowActivities{}

	for _, activity := range o.readyToRunActivities {
		response := make([]ResponseStartSchedule, 0)

		nodes, err := o.nodeRepository.GetNodesByRuntime(o.workflow.Spec.Runtime)
		if err != nil {
			config.App().Logger.Error("Error getting nodes: " + err.Error())
			return nil, err
		}
		for _, node := range nodes {

			nodeMetrics, err := o.nodeCurrentMetricsService.
				SetNodeName(node.Name).
				SetCurrentScheduled(newReadyToRunActivities).
				GetCurrentMetrics()

			if err != nil {
				config.App().Logger.Error("Error getting activity schedule by node name: " + err.Error())
				return nil, err
			}

			input := map[string]any{
				"time_estimate":   1.0,
				"memory_required": activity.GetMemoryRequired(),
				"vcpus_required":  activity.GetCpuRequired(),
				"memory_free":     nodeMetrics.GetMemoryFree(),
				"memory_max":      nodeMetrics.GetMemoryMax(),
				"vcpus_available": nodeMetrics.GetCpuFree(),
				"alpha":           0.0,
				"activity_name":   activity.GetName(),
			}

			akoScore, _ := o.StartRunSchedule(scheduleName, input)
			response = append(response, ResponseStartSchedule{
				"activity_id": activity.Id,
				"node_name":   node.Name,
				"ako_score":   akoScore,
				"input":       input,
			})

		}

		bestNode := o.getBestNode(response)

		metadataMap := map[string]any{
			"cpu":          activity.GetCpuRequired(),
			"memory":       activity.GetMemoryRequired(),
			"currentScore": bestNode["ako_score"].(float64),
			"othersScores": response,
		}

		metadataByte, err := json.Marshal(metadataMap)
		if err != nil {
			config.App().Logger.Error("Error marshalling metadata: " + err.Error())
			return nil, err
		}

		metadata := string(metadataByte)

		o.activityRepository.SetActivitySchedule(
			activity.WorkflowId,
			activity.Id,
			bestNode["node_name"].(string),
			scheduleName,
			activity.GetCpuRequired(),
			activity.GetMemoryRequired(),
			metadata,
		)

		newReadyToRunActivities = append(newReadyToRunActivities, activity)
	}

	return newReadyToRunActivities, nil

}

func (o *OrchestrateScheduleService) getBestNode(response []ResponseStartSchedule) ResponseStartSchedule {
	if len(response) == 0 {
		return nil
	}
	bestNode := response[0]

	for _, res := range response {
		if res["ako_score"].(float64) > bestNode["ako_score"].(float64) {
			bestNode = res
		}
	}

	return bestNode
}

func (r *OrchestrateScheduleService) StartRunSchedule(scheduleName string, input map[string]any) (float64, error) {
	// Here you would implement the logic to start running the schedule
	// For example, you might want to fetch the schedule by name and then execute it with the provided input

	schedule, err := r.scheduleRepository.GetScheduleByName(scheduleName)

	if err != nil {
		config.App().Logger.Error("Error getting schedule: " + err.Error())
		return 0, err
	}

	println("Schedule found: ", schedule.Name)

	p, err := plugin.Open(filepath.Clean(schedule.PluginSoPath))

	if err != nil {
		fmt.Println("Erro ao abrir plugin:", err)
		return 0, err
	}

	sym, err := p.Lookup("AkoScore")
	if err != nil {
		fmt.Println("Erro ao procurar símbolo 'AkoScore':", err)
		return 0, err
	}

	akoScoreFunc, ok := sym.(func(any) float64)

	if !ok {
		fmt.Println("Símbolo 'AkoScore' não é uma função válida")
		return 0, fmt.Errorf("invalid AkoScore function")
	}

	result := akoScoreFunc(input)

	return result, nil
}
