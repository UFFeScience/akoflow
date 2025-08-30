package activity_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/model"
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
)

type ActivityRepository struct {
	tableNameActivity             string
	tableNameActivityDependencies string
	tableNamePreActivity          string
	tableNameActivitySchedule     string
}

var StatusCreated = 0
var StatusRunning = 1
var StatusFinished = 2
var StatusCompleted = 3

func New() IActivityRepository {

	database := repository.Database{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, model.Activity{})
	if err != nil {
		return nil
	}
	c.Close()

	c = database.Connect()
	err = repository.CreateOrVerifyTable(c, model.ActivityDependency{})
	if err != nil {
		return nil
	}

	c.Close()

	c = database.Connect()
	err = repository.CreateOrVerifyTable(c, model.PreActivity{})

	if err != nil {
		return nil
	}
	c.Close()

	c = database.Connect()
	err = repository.CreateOrVerifyTable(c, model.ActivitySchedule{})
	if err != nil {
		return nil
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return &ActivityRepository{
		tableNameActivity:             model.Activity{}.TableName(),
		tableNameActivityDependencies: model.ActivityDependency{}.TableName(),
		tableNamePreActivity:          model.PreActivity{}.TableName(),
		tableNameActivitySchedule:     model.ActivitySchedule{}.TableName(),
	}
}

type IActivityRepository interface {
	Create(namespace string, workflow workflow_entity.Workflow, activities []workflow_activity_entity.WorkflowActivities) error
	GetActivitiesByWorkflowIds(ids []int) (ResultGetActivitiesByWorkflowIds, error)
	UpdateStatus(id int, status int) error
	UpdateProcID(id int, pid string) error
	Find(id int) (workflow_activity_entity.WorkflowActivities, error)
	GetByWorkflowId(id int) ([]workflow_activity_entity.WorkflowActivities, error)
	GetWfaDependencies(workflowId int) ([]workflow_activity_entity.WorkflowActivityDependencyDatabase, error)
	FindPreActivity(id int) (workflow_activity_entity.WorkflowPreActivityDatabase, error)
	UpdatePreActivity(id int, preactivity workflow_activity_entity.WorkflowPreActivityDatabase) error
	GetPreactivitiesCompleted() ([]workflow_activity_entity.WorkflowPreActivityDatabase, error)
	UpdateNodeSelector(id int, nodeSelector string) error
	SetActivitySchedule(workflowId int, activity int, nodeName string, scheduleName string, cpuRequired float64, memoryRequired float64, metadata string) error
	GetActivityScheduleByNodeName(nodeName string) ([]model.ActivitySchedule, error)
	GetAllRunningActivities() ([]workflow_activity_entity.WorkflowActivities, error)
	GetActivityScheduleByActivityId(activityId int) (model.ActivitySchedule, error)
	IsActivityScheduled(workflowId int, activityId int) (bool, error)
}
