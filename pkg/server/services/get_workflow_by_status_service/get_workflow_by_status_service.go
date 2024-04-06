package get_workflow_by_status_service

import (
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow_entity"
)

type GetWorkflowByStatusService struct {
}

func New() *GetWorkflowByStatusService {
	return &GetWorkflowByStatusService{}
}

func (o *GetWorkflowByStatusService) GetActivitiesByStatus(wfs workflow_entity.Workflow, status int) []workflow_activity_entity.WorkflowActivities {
	var wfsSelected []workflow_activity_entity.WorkflowActivities
	for _, activity := range wfs.Spec.Activities {
		if activity.Status == status {
			wfsSelected = append(wfsSelected, activity)
		}
	}
	return wfsSelected
}
