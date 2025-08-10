package node_current_metrics_service

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/node_repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
)

type NodeCurrentMetrics struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	CPUTotal    float64 `json:"cpu_total"`
	MemoryTotal float64 `json:"memory_total"`
}

func (n *NodeCurrentMetrics) GetCpuUsage() float64 {
	return n.CPUUsage
}

func (n *NodeCurrentMetrics) GetMemoryFree() float64 {
	return n.MemoryTotal - n.MemoryUsage
}

func (n *NodeCurrentMetrics) GetCpuFree() float64 {
	return n.CPUTotal - n.CPUUsage
}

func (n *NodeCurrentMetrics) GetMemoryMax() float64 {
	return n.MemoryTotal
}

func (n *NodeCurrentMetrics) GetCpuMax() float64 {
	return n.CPUTotal
}

type NodeCurrentMetricsService struct {
	nodeName string

	activityRepository activity_repository.IActivityRepository
	nodeRepository     node_repository.INodeRepository

	currentScheduled []workflow_activity_entity.WorkflowActivities
}

func New() NodeCurrentMetricsService {
	return NodeCurrentMetricsService{
		nodeName: "",

		nodeRepository:     node_repository.New(),
		activityRepository: activity_repository.New(),
		currentScheduled:   []workflow_activity_entity.WorkflowActivities{},
	}
}

func (n *NodeCurrentMetricsService) SetCurrentScheduled(activities []workflow_activity_entity.WorkflowActivities) *NodeCurrentMetricsService {
	n.currentScheduled = activities
	return n
}

func (n *NodeCurrentMetricsService) SetNodeName(nodeName string) *NodeCurrentMetricsService {
	n.nodeName = nodeName
	return n
}

func (n *NodeCurrentMetricsService) GetNodeName() string {
	return n.nodeName
}

func (n *NodeCurrentMetricsService) GetCurrentMetrics() (*NodeCurrentMetrics, error) {

	if n.nodeName == "" {
		return nil, fmt.Errorf("node name is required")
	}

	node, err := n.nodeRepository.GetByName(n.nodeName)

	if err != nil || node == nil {
		return nil, fmt.Errorf("node not found")
	}

	activitiesScheduleds, err := n.activityRepository.GetActivityScheduleByNodeName(n.nodeName)
	if err != nil {
		return nil, fmt.Errorf("error getting activity schedules: %v", err)
	}

	activitiesRunning, err := n.activityRepository.GetAllRunningActivities()
	if err != nil {
		return nil, fmt.Errorf("error getting running activities: %v", err)
	}

	activitiesScheduledsMap := make(map[int]map[string]bool)

	for _, activityScheduled := range activitiesScheduleds {
		if _, exists := activitiesScheduledsMap[activityScheduled.ActivityID]; !exists {
			activitiesScheduledsMap[activityScheduled.ActivityID] = map[string]bool{
				"activitySchedule": true,
				"activityRunning":  false,
			}
		} else {
			activitiesScheduledsMap[activityScheduled.ActivityID]["activitySchedule"] = true
		}
	}

	for _, activityRunning := range activitiesRunning {
		if _, exists := activitiesScheduledsMap[activityRunning.Id]; !exists {
			activitiesScheduledsMap[activityRunning.Id] = map[string]bool{
				"activitySchedule": false,
				"activityRunning":  true,
			}
		} else {
			activitiesScheduledsMap[activityRunning.Id]["activityRunning"] = true
		}
	}

	for _, activityRunning := range n.currentScheduled {
		activitiesScheduledsMap[activityRunning.Id] = map[string]bool{
			"activitySchedule": true,
			"activityRunning":  true,
		}
	}

	nodeCurrentMetrics := NodeCurrentMetrics{}

	for _, activityScheduled := range activitiesScheduleds {

		if activitiesScheduledsMap[activityScheduled.ActivityID]["activityRunning"] && activitiesScheduledsMap[activityScheduled.ActivityID]["activitySchedule"] {
			nodeCurrentMetrics.CPUUsage += activityScheduled.CpuRequired
			nodeCurrentMetrics.MemoryUsage += activityScheduled.MemoryRequired
		}

	}
	nodeCurrentMetrics.CPUTotal = node.CPUMax
	nodeCurrentMetrics.MemoryTotal = node.MemoryLimit

	return &nodeCurrentMetrics, nil
}
