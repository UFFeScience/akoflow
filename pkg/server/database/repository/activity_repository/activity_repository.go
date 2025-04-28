package activity_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/database/repository"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
)

type ActivityRepository struct {
	tableNameActivity             string
	tableNameActivityDependencies string
	tableNamePreActivity          string
}

var StatusCreated = 0
var StatusRunning = 1
var StatusFinished = 2
var StatusCompleted = 3

var TableNameActivities = "activities"
var ColumnsActivities = "(id INTEGER PRIMARY KEY AUTOINCREMENT, workflow_id INTEGER, namespace TEXT, name TEXT, image TEXT, resource_k8s_base64 TEXT, status INTEGER, proc_id TEXT, created_at TEXT, started_at TEXT, finished_at TEXT)"

var TableNameActivitiesDependencies = "activities_dependencies"
var ColumnsActivitiesDependencies = "(id INTEGER PRIMARY KEY AUTOINCREMENT, workflow_id INTEGER, activity_id INTEGER, depend_on_activity INTEGER)"

var TableNamePreActivities = "pre_activities"
var ColumnsPreActivities = "(id INTEGER PRIMARY KEY AUTOINCREMENT, activity_id INTEGER, workflow_id INTEGER, namespace TEXT, name TEXT, resource_k8s_base64 TXT, status INTEGER, log TEXT)"

func New() IActivityRepository {

	database := repository.Database{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, TableNameActivities, ColumnsActivities)
	if err != nil {
		return nil
	}
	c.Close()

	c = database.Connect()
	err = repository.CreateOrVerifyTable(c, TableNameActivitiesDependencies, ColumnsActivitiesDependencies)

	c.Close()

	c = database.Connect()
	err = repository.CreateOrVerifyTable(c, TableNamePreActivities, ColumnsPreActivities)

	if err != nil {
		return nil
	}

	err = c.Close()
	if err != nil {
		return nil
	}

	return &ActivityRepository{
		tableNameActivity:             TableNameActivities,
		tableNameActivityDependencies: TableNameActivitiesDependencies,
		tableNamePreActivity:          TableNamePreActivities,
	}
}

type IActivityRepository interface {
	Create(namespace string, workflowId int, image string, activities []workflow_activity_entity.WorkflowActivities) error
	GetActivitiesByWorkflowIds(ids []int) (ResultGetActivitiesByWorkflowIds, error)
	UpdateStatus(id int, status int) error
	UpdateProcID(id int, pid string) error
	Find(id int) (workflow_activity_entity.WorkflowActivities, error)
	GetByWorkflowId(id int) ([]workflow_activity_entity.WorkflowActivities, error)
	GetWfaDependencies(workflowId int) ([]workflow_activity_entity.WorkflowActivityDependencyDatabase, error)
	FindPreActivity(id int) (workflow_activity_entity.WorkflowPreActivityDatabase, error)
	UpdatePreActivity(id int, preactivity workflow_activity_entity.WorkflowPreActivityDatabase) error
	GetPreactivitiesCompleted() ([]workflow_activity_entity.WorkflowPreActivityDatabase, error)
}
