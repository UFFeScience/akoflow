package run_activity_in_cluster_service

//
//import (
//	"errors"
//	"github.com/ovvesley/akoflow/pkg/server/connector/connector_namespace_k8s"
//	"github.com/ovvesley/akoflow/pkg/server/connector/connector_pod_k8s"
//	"github.com/ovvesley/akoflow/pkg/server/connector/connector_pvc_k8s"
//	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
//	"github.com/ovvesley/akoflow/test/mock"
//	"go.uber.org/mock/gomock"
//	"testing"
//)
//
//func TestRunActivityInClusterService_handleGetOrCreateNamespaceCreatedNamespace(t *testing.T) {
//
//	namespace := "k8s-test"
//	mockCtrl := gomock.NewController(t)
//	defer mockCtrl.Finish()
//
//	workflowRepository := mock.NewMockIWorkflowRepository(mockCtrl)
//	activityRepository := mock.NewMockIActivityRepository(mockCtrl)
//
//	namespaceConnectorMock := mock.NewMockIConnectorNamespace(mockCtrl)
//	namespaceConnectorMock.
//		EXPECT().
//		GetNamespace(namespace).
//		Return(connector_namespace_k8s.ResponseGetNamespace{
//			Metadata: connector_namespace_k8s.ResponseGetNamespaceMetadata{
//				Name: "k8s-test",
//			},
//		}, nil)
//
//	connector := mock.NewMockIConnector(mockCtrl)
//	connector.EXPECT().Namespace().Return(namespaceConnectorMock)
//
//	r := &RunActivityInClusterService{
//		namespace:          namespace,
//		workflowRepository: workflowRepository,
//		activityRepository: activityRepository,
//		connector:          connector,
//	}
//	response := r.handleGetOrCreateNamespace(namespace)
//
//	if response != namespace {
//		t.Errorf("Expected %s, got %s", namespace, response)
//	}
//}
//
//func TestRunActivityInClusterService_handleGetOrCreateNamespaceNotCreatedNamespace(t *testing.T) {
//
//	namespace := "k8s-test"
//	mockCtrl := gomock.NewController(t)
//	defer mockCtrl.Finish()
//
//	workflowRepository := mock.NewMockIWorkflowRepository(mockCtrl)
//	activityRepository := mock.NewMockIActivityRepository(mockCtrl)
//
//	namespaceConnectorMock := mock.NewMockIConnectorNamespace(mockCtrl)
//	namespaceConnectorMock.
//		EXPECT().
//		GetNamespace(namespace).
//		Return(connector_namespace_k8s.ResponseGetNamespace{}, errors.New("Namespace not found"))
//
//	namespaceConnectorMock.
//		EXPECT().
//		CreateNamespace(namespace).
//		AnyTimes().
//		Return(connector_namespace_k8s.ResponseCreateNamespace{
//			Metadata: connector_namespace_k8s.ResponseCreateNamespaceMetadata{
//				Name: namespace,
//			},
//		}, nil)
//
//	connector := mock.NewMockIConnector(mockCtrl)
//	connector.EXPECT().Namespace().AnyTimes().Return(namespaceConnectorMock)
//
//	r := &RunActivityInClusterService{
//		namespace:          namespace,
//		activityRepository: activityRepository,
//		connector:          connector,
//	}
//	response := r.handleGetOrCreateNamespace(namespace)
//
//	if response != namespace {
//		t.Errorf("Expected %s, got %s", namespace, response)
//	}
//}
//
//func TestRunActivityInClusterServiceSuccessFlow(t *testing.T) {
//
//	control := gomock.NewController(t)
//	defer control.Finish()
//
//	workflowId := 123
//	activityId := 456
//
//	activityRepositoryMock := mock.NewMockIActivityRepository(control)
//	activityRepositoryMock.
//		EXPECT().
//		Find(activityId).
//		AnyTimes().
//		Return(workflow_entity.WorkflowActivities{
//			Id:         activityId,
//			WorkflowId: workflowId,
//		}, nil)
//
//	activityRepositoryMock.
//		EXPECT().
//		UpdateStatus(gomock.Any(), gomock.Any()).
//		AnyTimes().
//		Return(nil)
//
//	workflowUsed := workflow_entity.Workflow{
//		Name: "Workflow de Teste",
//		Spec: workflow_entity.WorkflowSpec{
//			Image:            "alpine:latest",
//			Namespace:        "k8s-test",
//			StorageClassName: "hostpath",
//			StorageSize:      "1Gi",
//			MountPath:        "/mnt/data",
//			Activities:       nil,
//		},
//		Id: workflowId,
//	}
//	workflowRepositoryMock := mock.NewMockIWorkflowRepository(control)
//	workflowRepositoryMock.
//		EXPECT().
//		Find(workflowId).
//		AnyTimes().
//		Return(workflowUsed, nil)
//
//	workflowRepositoryMock.
//		EXPECT().
//		UpdateStatus(gomock.Any(), gomock.Any()).
//		AnyTimes().
//		Return(nil)
//
//	namespaceConnectorMock := mock.NewMockIConnectorNamespace(control)
//	namespaceConnectorMock.
//		EXPECT().
//		GetNamespace("k8s-test").
//		Return(connector_namespace_k8s.ResponseGetNamespace{
//			Metadata: connector_namespace_k8s.ResponseGetNamespaceMetadata{
//				Name: "k8s-test",
//			},
//		}, nil)
//
//	pvcConnectorMock := mock.NewMockIConnectorPvc(control)
//	pvcConnectorMock.
//		EXPECT().
//		GetPersistentVolumeClain(
//			workflowUsed.GetVolumeName(),
//			"k8s-test").
//		Return(connector_pvc_k8s.ResponseGetPersistentVolumeClain{Metadata: connector_pvc_k8s.ResponseGetPersistentVolumeClainMetadata{Name: workflowUsed.GetVolumeName()}}, nil)
//
//	jobConnectorMock := mock.NewMockIConnectorJob(control)
//	jobConnectorMock.
//		EXPECT().
//		ApplyJob(gomock.Any(), gomock.Any()).
//		Return(nil)
//
//	podConnectorMock := mock.NewMockIConnectorPod(control)
//	podConnectorMock.
//		EXPECT().
//		GetPodByJob(gomock.Any(), gomock.Any()).
//		Return(connector_pod_k8s.ResponseGetJobByPod{
//			Items: []connector_pod_k8s.ResponseGetJobByPodItem{
//				{
//					Metadata: connector_pod_k8s.ResponseGetJobByPodItemMetadata{
//						Name: "pod-test",
//					},
//				},
//			},
//		}, nil)
//
//	mockConnector := mock.NewMockIConnector(control)
//	mockConnector.EXPECT().Namespace().Return(namespaceConnectorMock)
//	mockConnector.EXPECT().PersistentVolumeClain().Return(pvcConnectorMock)
//	mockConnector.EXPECT().Job().Return(jobConnectorMock)
//	mockConnector.EXPECT().Pod().Return(podConnectorMock)
//
//	service := New(ParamsNewRunActivityInClusterService{
//		Namespace:          workflowUsed.Spec.Namespace,
//		WorkflowRepository: workflowRepositoryMock,
//		ActivityRepository: activityRepositoryMock,
//		Connector:          mockConnector,
//	})
//
//	service.Run(activityId)
//
//}
