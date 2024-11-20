package install_akoflow_local_service

import "github.com/ovvesley/akoflow/pkg/client/services/install_kubernetes_local_service"

type InstallAkoflowLocalService struct {
	installKubernetesService *install_kubernetes_local_service.InstallKubernetesLocalService
}

func New() *InstallAkoflowLocalService {
	return &InstallAkoflowLocalService{}
}

func (i *InstallAkoflowLocalService) Install() {
	// install kubernetes local
	i.installKubernetesService = install_kubernetes_local_service.New()
	i.installKubernetesService.Install()

	// install akoflow resources
}
