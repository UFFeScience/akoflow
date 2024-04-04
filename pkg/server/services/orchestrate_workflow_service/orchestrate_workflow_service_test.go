package orchestrate_workflow_service

import (
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow"
	"github.com/ovvesley/scik8sflow/pkg/server/repository/activity_repository"
	"testing"
)

func TestOrchestrateWorflowService_OrchestrateCreatedWorkflow(t *testing.T) {
	t.Log("TestOrchestrateWorflowService_OrchestrateCreatedWorkflow")

	wf := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{
				{
					Name:      "A",
					Id:        1,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-1'",
					DependsOn: []string{},
				},
				{
					Name:      "B",
					Id:        2,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-2'",
					DependsOn: []string{"A"},
				},
				{
					Name:      "C",
					Id:        3,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-3'",
					DependsOn: []string{"A", "B"},
				},
			},
		},
	}

	service := New()
	mapWfWfaDispatched := service.Orchestrate([]workflow.Workflow{wf, wf})

	// assert
	if len(mapWfWfaDispatched) != 1 {
		t.Errorf("Expected 1, got %d", len(mapWfWfaDispatched))
	}

	if len(mapWfWfaDispatched[wf.Id]) != 1 {
		t.Errorf("Expected 1, got %d", len(mapWfWfaDispatched[wf.Id]))
	}

	if mapWfWfaDispatched[wf.Id][0].Name != "A" {
		t.Errorf("Expected A, got %s", mapWfWfaDispatched[wf.Id][0].Name)
	}

	if mapWfWfaDispatched[wf.Id][0].Id != 1 {
		t.Errorf("Expected 1, got %d", mapWfWfaDispatched[wf.Id][0].Id)
	}

	if mapWfWfaDispatched[wf.Id][0].Run != "echo 'activity-1'" {
		t.Errorf("Expected echo 'activity-1', got %s", mapWfWfaDispatched[wf.Id][0].Run)
	}
}

func TestOrchestrateWorflowService_OrchestrateRunningWorkflow(t *testing.T) {
	t.Log("TestOrchestrateWorflowService_OrchestrateRunningWorkflow")

	wf := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{
				{
					Name:      "A",
					Id:        1,
					Status:    activity_repository.StatusRunning,
					Run:       "echo 'activity-1'",
					DependsOn: []string{},
				},
				{
					Name:      "B",
					Id:        2,
					Status:    activity_repository.StatusRunning,
					Run:       "echo 'activity-2'",
					DependsOn: []string{"A"},
				},
				{
					Name:      "C",
					Id:        3,
					Status:    activity_repository.StatusRunning,
					Run:       "echo 'activity-3'",
					DependsOn: []string{"A", "B"},
				},
			},
		},
	}

	service := New()
	mapWfWfaDispatched := service.Orchestrate([]workflow.Workflow{wf, wf})

	// assert
	if len(mapWfWfaDispatched[wf.Id]) != 0 {
		t.Errorf("Expected 0, got %d", len(mapWfWfaDispatched))
	}

}

func TestOrchestrateWorflowService_OrchestrateFinishedWorkflow(t *testing.T) {
	t.Log("TestOrchestrateWorflowService_OrchestrateFinishedWorkflow")

	wf := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{
				{
					Name:      "A",
					Id:        1,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-1'",
					DependsOn: []string{},
				},
				{
					Name:      "B",
					Id:        2,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-2'",
					DependsOn: []string{"A"},
				},
				{
					Name:      "C",
					Id:        3,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-3'",
					DependsOn: []string{"A", "B"},
				},
			},
		},
	}

	service := New()
	mapWfWfaDispatched := service.Orchestrate([]workflow.Workflow{wf, wf})

	// assert
	if len(mapWfWfaDispatched[wf.Id]) != 0 {
		t.Errorf("Expected 0, got %d", len(mapWfWfaDispatched))
	}

}

func TestOrchestrateWorflowService_OrchestrateNilWorkflow(t *testing.T) {
	t.Log("TestOrchestrateWorflowService_OrchestrateFinishedWorkflow")

	wf := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{
				{
					Name:      "A",
					Id:        1,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-1'",
					DependsOn: []string{},
				},
				{
					Name:      "B",
					Id:        2,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-2'",
					DependsOn: []string{"A"},
				},
				{
					Name:      "C",
					Id:        3,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-3'",
					DependsOn: nil,
				},
			},
		},
	}

	service := New()
	mapWfWfaDispatched := service.Orchestrate([]workflow.Workflow{wf, wf})

	// assert
	if len(mapWfWfaDispatched[wf.Id]) != 2 {
		t.Errorf("Expected 2, got %d", len(mapWfWfaDispatched))
	}

	if mapWfWfaDispatched[wf.Id][0].Name != "B" {
		t.Errorf("Expected B, got %s", mapWfWfaDispatched[wf.Id][0].Name)
	}

	if mapWfWfaDispatched[wf.Id][1].Name != "C" {
		t.Errorf("Expected C, got %s", mapWfWfaDispatched[wf.Id][1].Name)
	}

}

func TestOrchestrateWorflowService_OrchestrateMixedWorkflow(t *testing.T) {
	t.Log("TestOrchestrateWorflowService_OrchestrateMixedWorkflow")

	wf := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{
				{
					Name:      "A",
					Id:        1,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-1'",
					DependsOn: []string{},
				},
				{
					Name:      "B",
					Id:        2,
					Status:    activity_repository.StatusRunning,
					Run:       "echo 'activity-2'",
					DependsOn: []string{"A"},
				},
				{
					Name:      "C",
					Id:        3,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-3'",
					DependsOn: []string{"A", "B"},
				},
			},
		},
	}

	service := New()
	mapWfWfaDispatched := service.Orchestrate([]workflow.Workflow{wf, wf})

	// assert
	if len(mapWfWfaDispatched[wf.Id]) != 0 {
		t.Errorf("Expected 0, got %d", len(mapWfWfaDispatched))
	}

}

func TestOrchestrateWorflowService_OrchestrateMixedWorkflow2(t *testing.T) {
	t.Log("TestOrchestrateWorflowService_OrchestrateMixedWorkflow2")

	wf := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{
				{
					Name:      "A",
					Id:        1,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-1'",
					DependsOn: []string{},
				},
				{
					Name:      "B",
					Id:        2,
					Status:    activity_repository.StatusRunning,
					Run:       "echo 'activity-2'",
					DependsOn: []string{"A"},
				},
				{
					Name:      "C",
					Id:        3,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-3'",
					DependsOn: []string{"A", "B"},
				},
				{
					Name:      "D",
					Id:        4,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-4'",
					DependsOn: []string{},
				},
			},
		},
	}

	service := New()
	mapWfWfaDispatched := service.Orchestrate([]workflow.Workflow{wf, wf})

	// assert
	if len(mapWfWfaDispatched[wf.Id]) != 1 {
		t.Errorf("Expected 1, got %d", len(mapWfWfaDispatched))
	}

	if mapWfWfaDispatched[wf.Id][0].Name != "D" {
		t.Errorf("Expected D, got %s", mapWfWfaDispatched[wf.Id][0].Name)
	}

	if mapWfWfaDispatched[wf.Id][0].Id != 4 {
		t.Errorf("Expected 4, got %d", mapWfWfaDispatched[wf.Id][0].Id)
	}

}

func TestOrchestrateWorflowService_OrchestrateMixedWorkflow3(t *testing.T) {
	t.Log("TestOrchestrateWorflowService_OrchestrateMixedWorkflow3")

	wf := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{
				{
					Name:      "A",
					Id:        1,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-1'",
					DependsOn: []string{},
				},
				{
					Name:      "B",
					Id:        2,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-2'",
					DependsOn: []string{},
				},
				{
					Name:      "C",
					Id:        3,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-3'",
					DependsOn: []string{},
				},
				{
					Name:      "D",
					Id:        4,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-4'",
					DependsOn: []string{"A", "B", "C"},
				},

				{
					Name:      "E",
					Id:        5,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-5'",
					DependsOn: []string{"C"},
				},

				{
					Name:      "F",
					Id:        6,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-6'",
					DependsOn: []string{"D", "E"},
				},

				{
					Name:      "G",
					Id:        7,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-7'",
					DependsOn: []string{"F"},
				},
			},
		},
	}

	service := New()
	mapWfWfaDispatched := service.Orchestrate([]workflow.Workflow{wf, wf})

	// assert

	if len(mapWfWfaDispatched[wf.Id]) != 3 {
		t.Errorf("Expected 3, got %d", len(mapWfWfaDispatched[wf.Id]))
	}

	if mapWfWfaDispatched[wf.Id][0].Name != "A" {
		t.Errorf("Expected A, got %s", mapWfWfaDispatched[wf.Id][0].Name)
	}

	if mapWfWfaDispatched[wf.Id][1].Name != "B" {
		t.Errorf("Expected B, got %s", mapWfWfaDispatched[wf.Id][1].Name)
	}

	if mapWfWfaDispatched[wf.Id][2].Name != "C" {
		t.Errorf("Expected C, got %s", mapWfWfaDispatched[wf.Id][2].Name)
	}

}

func TestOrchestrateWorflowService_OrchestrateMixedWorkflow4(t *testing.T) {
	t.Log("TestOrchestrateWorflowService_OrchestrateMixedWorkflow4")

	wf := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{
				{
					Name:      "A",
					Id:        1,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-1'",
					DependsOn: []string{},
				},
				{
					Name:      "B",
					Id:        2,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-2'",
					DependsOn: []string{},
				},
				{
					Name:      "C",
					Id:        3,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-3'",
					DependsOn: []string{},
				},
				{
					Name:      "D",
					Id:        4,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-4'",
					DependsOn: []string{"A", "B", "C"},
				},

				{
					Name:      "E",
					Id:        5,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-5'",
					DependsOn: []string{"C"},
				},

				{
					Name:      "F",
					Id:        6,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-6'",
					DependsOn: []string{"D", "E"},
				},

				{
					Name:      "G",
					Id:        7,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-7'",
					DependsOn: []string{"F"},
				},
			},
		},
	}

	service := New()
	mapWfWfaDispatched := service.Orchestrate([]workflow.Workflow{wf, wf})

	// assert

	if len(mapWfWfaDispatched[wf.Id]) != 2 {
		t.Errorf("Expected 2, got %d", len(mapWfWfaDispatched[wf.Id]))
	}

	if mapWfWfaDispatched[wf.Id][0].Name != "D" {
		t.Errorf("Expected D, got %s", mapWfWfaDispatched[wf.Id][0].Name)
	}

	if mapWfWfaDispatched[wf.Id][1].Name != "E" {
		t.Errorf("Expected E, got %s", mapWfWfaDispatched[wf.Id][1].Name)
	}

}

func TestOrchestrateWorflowService_OrchestrateMixedWorkflow5(t *testing.T) {
	t.Log("TestOrchestrateWorflowService_OrchestrateMixedWorkflow5")

	wf := workflow.Workflow{
		Spec: workflow.WorkflowSpec{
			Activities: []workflow.WorkflowActivities{
				{
					Name:      "A",
					Id:        1,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-1'",
					DependsOn: []string{},
				},
				{
					Name:      "B",
					Id:        2,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-2'",
					DependsOn: []string{},
				},
				{
					Name:      "C",
					Id:        3,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-3'",
					DependsOn: []string{},
				},
				{
					Name:      "D",
					Id:        4,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-4'",
					DependsOn: []string{"A", "B", "C"},
				},

				{
					Name:      "E",
					Id:        5,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-5'",
					DependsOn: []string{"C"},
				},

				{
					Name:      "F",
					Id:        6,
					Status:    activity_repository.StatusFinished,
					Run:       "echo 'activity-6'",
					DependsOn: []string{"D", "E"},
				},

				{
					Name:      "G",
					Id:        7,
					Status:    activity_repository.StatusCreated,
					Run:       "echo 'activity-7'",
					DependsOn: []string{"F"},
				},
			},
		},
	}

	service := New()
	mapWfWfaDispatched := service.Orchestrate([]workflow.Workflow{wf, wf})

	// assert

	if len(mapWfWfaDispatched[wf.Id]) != 1 {
		t.Errorf("Expected 1, got %d", len(mapWfWfaDispatched[wf.Id]))
	}

	if mapWfWfaDispatched[wf.Id][0].Name != "G" {
		t.Errorf("Expected G, got %s", mapWfWfaDispatched[wf.Id][0].Name)
	}

}
