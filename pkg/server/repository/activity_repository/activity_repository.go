package activity_repository

import (
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow"
	"github.com/ovvesley/scik8sflow/pkg/server/repository"
)

type ActivityRepository struct {
	tableNameActivity             string
	tableNameActivityDependencies string
}

var StatusCreated = 0
var StatusRunning = 1
var StatusFinished = 2

var TableNameActivities = "activities"
var ColumnsActivities = "(id INTEGER PRIMARY KEY AUTOINCREMENT, workflow_id INTEGER, namespace TEXT, name TEXT, image TEXT, resource_k8s_base64 TEXT, status INTEGER)"

var TableNameActivitiesDependencies = "activities_dependencies"
var ColumnsActivitiesDependencies = "(id INTEGER PRIMARY KEY AUTOINCREMENT, workflow_id INTEGER, activity_id INTEGER, depend_on_activity INTEGER)"

func New() IActivityRepository {

	database := repository.Database{}
	c := database.Connect()
	err := repository.CreateOrVerifyTable(c, TableNameActivities, ColumnsActivities)
	if err != nil {
		return nil
	}

	c = database.Connect()
	err = repository.CreateOrVerifyTable(c, TableNameActivitiesDependencies, ColumnsActivitiesDependencies)

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
	}
}

type IActivityRepository interface {
	Create(namespace string, workflowId int, image string, activities []workflow.WorkflowActivities) error
	GetActivitiesByWorkflowIds(ids []int) (ResultGetActivitiesByWorkflowIds, error)
	UpdateStatus(id int, status int) error
	Find(id int) (workflow.WorkflowActivities, error)
	GetByWorkflowId(id int) ([]workflow.WorkflowActivities, error)
}
