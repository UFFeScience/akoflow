package create_pvc_service

import (
	"github.com/ovvesley/akoflow/pkg/server/connector"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/repository/storages_repository"
)

type CreatePVCService struct {
	connector         connector.IConnector
	storageRepository storages_repository.IStorageRepository
}
type ParamsNewCreatePVCService struct {
	Connector         connector.IConnector
	storageRepository storages_repository.IStorageRepository
}

func New(params ...ParamsNewCreatePVCService) CreatePVCService {

	if len(params) > 0 {
		return CreatePVCService{
			connector:         params[0].Connector,
			storageRepository: params[0].storageRepository,
		}
	}

	return CreatePVCService{
		connector:         connector.New(),
		storageRepository: storages_repository.New(),
	}
}

func (c *CreatePVCService) GetOrCreatePersistentVolumeClainByActivity(wf workflow_entity.Workflow, wfa workflow_activity_entity.WorkflowActivities, namespace string) (string, error) {
	pvc, err := c.connector.PersistentVolumeClain().GetPersistentVolumeClain(wfa.GetVolumeName(), namespace)

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

	pv, err := c.connector.PersistentVolumeClain().CreatePersistentVolumeClain(wfa.GetVolumeName(), namespace, wf.Spec.StorageSize, wf.Spec.StorageClassName)

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
