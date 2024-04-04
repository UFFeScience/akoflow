package get_workflow_by_status_service

import (
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow"
	"github.com/ovvesley/scik8sflow/pkg/server/repository/activity_repository"
	"testing"
)

func TestGetWorkflowByStatusService_GetActivitiesByStatusRunning(t *testing.T) {
	// Arrange
	service := New()
	wf := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{
				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusCreated,
				},

				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusCreated,
				},
			},
		},
	}

	// Act
	result := service.GetActivitiesByStatus(wf, activity_repository.StatusRunning)

	// Assert
	if len(result) != 4 {
		t.Errorf("Expected 4, got %d", len(result))
	}
}

func TestGetWorkflowByStatusService_GetActivitiesByStatusCreated(t *testing.T) {
	// Arrange
	service := New()
	workflow := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{
				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusCreated,
				},

				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusCreated,
				},
			},
		},
	}

	// Act
	result := service.GetActivitiesByStatus(workflow, activity_repository.StatusCreated)

	// Assert
	if len(result) != 2 {
		t.Errorf("Expected 2, got %d", len(result))
	}
}

func TestGetWorkflowByStatusService_GetActivitiesByStatusFinished(t *testing.T) {
	// Arrange
	service := New()
	wf := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{
				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusCreated,
				},

				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusCreated,
				},
			},
		},
	}

	// Act
	result := service.GetActivitiesByStatus(wf, activity_repository.StatusFinished)

	// Assert
	if len(result) != 0 {
		t.Errorf("Expected 0, got %d", len(result))
	}
}

func TestGetWorkflowByStatusService_GetActivitiesByStatusEmpty(t *testing.T) {
	// Arrange
	service := New()
	wf := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{
				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusCreated,
				},

				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusRunning,
				},
				{
					Status: activity_repository.StatusCreated,
				},
			},
		},
	}

	// Act
	result := service.GetActivitiesByStatus(wf, 999)

	// Assert
	if len(result) != 0 {
		t.Errorf("Expected 0, got %d", len(result))
	}
}

func TestGetWorkflowByStatusService_GetActivitiesByStatusEmptyWorkflow(t *testing.T) {
	// Arrange
	service := New()
	wf := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{},
		},
	}

	// Act
	result := service.GetActivitiesByStatus(wf, activity_repository.StatusCreated)

	// Assert
	if len(result) != 0 {
		t.Errorf("Expected 0, got %d", len(result))
	}
}

func TestGetWorkflowByStatusService_GetActivitiesByStatusNilWorkflow(t *testing.T) {
	// Arrange
	service := New()

	// Act
	result := service.GetActivitiesByStatus(workflow.Workflow{}, activity_repository.StatusCreated)

	// Assert
	if len(result) != 0 {
		t.Errorf("Expected 0, got %d", len(result))
	}
}

func TestGetWorkflowByStatusService_GetActivitiesByStatusNilWorkflowSpec(t *testing.T) {
	// Arrange
	service := New()
	wf := workflow.Workflow{}

	// Act
	result := service.GetActivitiesByStatus(wf, activity_repository.StatusCreated)

	// Assert
	if len(result) != 0 {
		t.Errorf("Expected 0, got %d", len(result))
	}
}
