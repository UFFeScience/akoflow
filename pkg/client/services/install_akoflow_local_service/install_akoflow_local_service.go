package install_akoflow_local_service

import (
	"github.com/ovvesley/akoflow/pkg/client/services/install_kubernetes_local_service"
	"github.com/ovvesley/akoflow/pkg/client/services/ssh_connection_service"
)

type InstallAkoflowLocalService struct {
	installKubernetesService *install_kubernetes_local_service.InstallKubernetesLocalService
	sshConnectionService     *ssh_connection_service.SSHConnectionService
}

func New() *InstallAkoflowLocalService {
	return &InstallAkoflowLocalService{
		installKubernetesService: nil,
		sshConnectionService:     nil,
	}
}

func (i *InstallAkoflowLocalService) SetSSHConnectionService(sshConnectionService *ssh_connection_service.SSHConnectionService) *InstallAkoflowLocalService {
	i.sshConnectionService = sshConnectionService
	return i
}

func (i *InstallAkoflowLocalService) Install() {

	i.sshConnectionService.EstablishConnectionWithHosts()
	i.sshConnectionService.ExecuteCommands([]string{
		"apt update",
		"sleep 5",
	})

	i.sshConnectionService.ExecuteCommands([]string{
		"help",
	})

	i.sshConnectionService.CloseConnections()

}
