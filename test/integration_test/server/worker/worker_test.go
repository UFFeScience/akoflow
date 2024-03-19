package worker

import (
	"github.com/ovvesley/scik8sflow/pkg/server/connector/connector_namespace_k8s"
	"github.com/ovvesley/scik8sflow/pkg/server/connector/connector_pvc_k8s"
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow"
	"github.com/ovvesley/scik8sflow/pkg/server/services/run_activity_in_cluster_service"
	"github.com/ovvesley/scik8sflow/test/mock"
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

	workflowUsed := workflow.Workflow{
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
	}
	workflowRepositoryMock := mock.NewMockIWorkflowRepository(control)
	workflowRepositoryMock.
		EXPECT().
		Find(workflowId).
		Return(workflowUsed, nil)

	namespaceConnectorMock := mock.NewMockIConnectorNamespace(control)
	namespaceConnectorMock.
		EXPECT().
		GetNamespace("k8s-test").
		Return(connector_namespace_k8s.ResponseGetNamespace{
			Metadata: connector_namespace_k8s.ResponseGetNamespaceMetadata{
				Name: "k8s-test",
			},
		}, nil)

	pvcConnectorMock := mock.NewMockIConnectorPvc(control)
	pvcConnectorMock.
		EXPECT().
		GetPersistentVolumeClain(
			workflowUsed.GetVolumeName(),
			"k8s-test").
		Return(connector_pvc_k8s.ResponseGetPersistentVolumeClain{Metadata: connector_pvc_k8s.ResponseGetPersistentVolumeClainMetadata{Name: workflowUsed.GetVolumeName()}}, nil)

	mockConnector := mock.NewMockIConnector(control)
	mockConnector.EXPECT().Namespace().Return(namespaceConnectorMock)
	mockConnector.EXPECT().PersistentVolumeClain().Return(pvcConnectorMock)

	service := run_activity_in_cluster_service.New(run_activity_in_cluster_service.ParamsNewRunActivityInClusterService{
		Namespace:          workflowUsed.Spec.Namespace,
		WorkflowRepository: workflowRepositoryMock,
		ActivityRepository: activityRepositoryMock,
		Connector:          mockConnector,
	})

	service.Run(activityId)

}
