package activity_repository

import (
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/model"
	"github.com/ovvesley/akoflow/pkg/server/repository"
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
	db := repository.GetInstance()

	if err := db.CreateOrVerifyTable(model.Activity{}); err != nil {
		println("Error creating activities table:", err.Error())
		return nil
	}

	if err := db.CreateOrVerifyTable(model.ActivityDependency{}); err != nil {
		println("Error creating activities_dependencies table:", err.Error())
		return nil
	}

	if err := db.CreateOrVerifyTable(model.PreActivity{}); err != nil {
		println("Error creating pre_activities table:", err.Error())
		return nil
	}

	return &ActivityRepository{
		tableNameActivity:             model.Activity{}.TableName(),
		tableNameActivityDependencies: model.ActivityDependency{}.TableName(),
		tableNamePreActivity:          model.PreActivity{}.TableName(),
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
