package kubernetes_runtime_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s"
	"github.com/ovvesley/akoflow/pkg/server/connector/connector_k8s/connector_pvc_k8s"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/storages_repository"
)

type CreatePVCService struct {
	connector         connector_k8s.IConnector
	storageRepository storages_repository.IStorageRepository
}

func NewCreatePVCService() CreatePVCService {
	return CreatePVCService{
		connector:         config.App().Connector.K8sConnector,
		storageRepository: config.App().Repository.StoragesRepository,
	}
}

func (c *CreatePVCService) GetOrCreatePersistentVolumeClainByActivity(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities, namespace string) (string, error) {

	var err error
	var pvc connector_pvc_k8s.ResponseGetPersistentVolumeClain

	if wf.IsStoragePolicyStandalone() {
		pvc, err = c.connector.PersistentVolumeClain().GetPersistentVolumeClain(wfa.GetVolumeName(), namespace)
	} else {
		pvc, err = c.connector.PersistentVolumeClain().GetPersistentVolumeClain(wf.MakeVolumeNameDistributed(), namespace)
	}

	if err != nil {
		println("Persistent volume not found")
		return c.handleCreatePersistentVolumeClain(wf, wfa, namespace)
	}

	err = c.storageRepository.Update(storages_repository.ParamsStorageUpdate{
		PvcName:    wfa.GetVolumeName(),
		Status:     storages_repository.StatusCreated,
		ActivityId: wfa.Id,
	})

	return pvc.Metadata.Name, nil
}

func (c *CreatePVCService) handleCreatePersistentVolumeClain(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities, namespace string) (string, error) {

	var pv connector_pvc_k8s.ResponseCreatePersistentVolumeClain
	var err error

	if wf.IsStoragePolicyStandalone() {
		pv, err = c.connector.
			PersistentVolumeClain().
			CreatePersistentVolumeClain(
				wfa.GetVolumeName(),
				namespace,
				wf.Spec.StoragePolicy.StorageSize,
				wf.Spec.StoragePolicy.StorageClassName,
			)
	} else {
		pv, err = c.connector.
			PersistentVolumeClain().
			CreatePersistentVolumeClain(
				wf.MakeVolumeNameDistributed(),
				namespace,
				wf.Spec.StoragePolicy.StorageSize,
				wf.MakeStorageClassNameDistributed(),
			)
	}

	if err != nil {
		println("Error creating persistent volume")
		return "", err
	}

	if pv.Metadata.Name == "" {
		println("Error creating persistent volume")
		return "", err
	}

	err = c.storageRepository.Update(storages_repository.ParamsStorageUpdate{
		PvcName:    wfa.GetVolumeName(),
		Status:     storages_repository.StatusCreated,
		ActivityId: wfa.Id,
	})

	if err != nil {
		return "", err
	}

	return pv.Metadata.Name, nil
}
