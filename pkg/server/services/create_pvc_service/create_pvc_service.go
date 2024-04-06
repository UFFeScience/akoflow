package create_pvc_service

import (
	"github.com/ovvesley/scik8sflow/pkg/server/connector"
	"github.com/ovvesley/scik8sflow/pkg/server/entities/workflow"
)

type CreatePVCService struct {
	connector connector.IConnector
}
type ParamsNewCreatePVCService struct {
	Connector connector.IConnector
}

func New(params ...ParamsNewCreatePVCService) CreatePVCService {

	if len(params) > 0 {
		return CreatePVCService{
			connector: params[0].Connector,
		}
	}

	return CreatePVCService{
		connector: connector.New(),
	}
}

func (c *CreatePVCService) GetOrCreatePersistentVolumeClainByActivity(wf workflow.Workflow, wfa workflow.WorkflowActivities, namespace string) (string, error) {
	pvc, err := c.connector.PersistentVolumeClain().GetPersistentVolumeClain(wfa.GetVolumeName(), namespace)

	if err != nil {
		println("Persistent volume not found")
		return c.handleCreatePersistentVolumeClain(wf, wfa, namespace)
	}

	return pvc.Metadata.Name, nil
}

func (c *CreatePVCService) handleCreatePersistentVolumeClain(wf workflow.Workflow, wfa workflow.WorkflowActivities, namespace string) (string, error) {

	pv, err := c.connector.PersistentVolumeClain().CreatePersistentVolumeClain(wfa.GetVolumeName(), namespace, wf.Spec.StorageSize, wf.Spec.StorageClassName)

	if err != nil {
		println("Error creating persistent volume")
		return "", err
	}

	if pv.Metadata.Name == "" {
		println("Error creating persistent volume")
		return "", err
	}

	return pv.Metadata.Name, nil
}
