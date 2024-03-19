package worker

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/entities/workflow"
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/services/run_activity_in_cluster_service"
	"github.com/ovvesley/scientific-workflow-k8s/test/mock"
	"go.uber.org/mock/gomock"
	"testing"
)

func TestRunActivityInClusterService(t *testing.T) {

	control := gomock.NewController(t)
	defer control.Finish()

	workflowId := 123
	activityId := 456

	activityRepositoryMock := mock.NewMockIActivityRepository(control)
	activityRepositoryMock.
		EXPECT().
		Find(activityId).
		Return(workflow.WorkflowActivities{
			Id:         activityId,
			WorkflowId: workflowId,
		}, nil)

	workflowRepositoryMock := mock.NewMockIWorkflowRepository(control)
	workflowRepositoryMock.
		EXPECT().
		Find(workflowId).
		Return(workflow.Workflow{
			Name: "Workflow de Teste",
			Spec: workflow.WorkflowSpec{
				Image:            "alpine:latest",
				Namespace:        "k8s-test",
				StorageClassName: "hostpath",
				StorageSize:      "1Gi",
				MountPath:        "/mnt/data",
				Activities:       nil,
			},
			Id: workflowId,
		}, nil)

	service := run_activity_in_cluster_service.New(run_activity_in_cluster_service.ParamsNewRunActivityInClusterService{
		Namespace:          "k8science-cluster-manager",
		WorkflowRepository: workflowRepositoryMock,
		ActivityRepository: activityRepositoryMock,
	})

	service.Run(activityId)

}
