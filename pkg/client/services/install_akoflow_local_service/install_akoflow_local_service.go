package install_akoflow_local_service

import (
	"github.com/ovvesley/akoflow/pkg/client/services/install_kubernetes_local_service"
	"github.com/ovvesley/akoflow/pkg/client/services/ssh_connection_service"
)

const (
	addKubernetesRepoCommand      = "echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.31/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list"
	addKubernetesKeyringCommand   = "sudo mkdir -p /etc/apt/keyrings && curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.31/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg"
	installPackagesCommand        = "sudo apt update && sudo apt install -y uidmap apt-transport-https ca-certificates curl gpg kubelet kubeadm kubectl"
	installDockerCommand          = "curl -fsSL https://get.docker.com -o get-docker.sh && sh get-docker.sh && dockerd-rootless-setuptool.sh install"
	holdKubernetesPackagesCommand = "sudo apt-mark hold kubelet kubeadm kubectl"
	configureContainerdCommand    = "sudo mkdir -p /etc/containerd && containerd config default | sudo tee /etc/containerd/config.toml && sudo systemctl restart containerd"
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

	i.sshConnectionService.ExecuteCommandsInMultipleHost([]string{
		addKubernetesRepoCommand,
		addKubernetesKeyringCommand,
		installPackagesCommand,
		installDockerCommand,
		holdKubernetesPackagesCommand,
		configureContainerdCommand,
	})

	mainHost := i.sshConnectionService.GetMainNode()

	i.sshConnectionService.ExecuteCommandsOnHost(mainHost, []string{"echo hello world >> test.txt"})

	i.sshConnectionService.CloseConnections()
}
