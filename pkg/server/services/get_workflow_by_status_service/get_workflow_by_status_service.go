package get_workflow_by_status_service

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
)

type GetWorkflowByStatusService struct {
}

func New() *GetWorkflowByStatusService {
	return &GetWorkflowByStatusService{}
}

func (o *GetWorkflowByStatusService) GetActivitiesByStatus(wfs workflow.Workflow, status int) []workflow.WorkflowActivities {
	var wfsSelected []workflow.WorkflowActivities
	for _, activity := range wfs.Spec.Activities {
		if activity.Status == status {
			wfsSelected = append(wfsSelected, activity)
		}
	}
	return wfsSelected
}
