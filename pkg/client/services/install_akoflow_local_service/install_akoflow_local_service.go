package install_akoflow_local_service

import (
	"fmt"
	"regexp"

	ssh_client_entity "github.com/ovvesley/akoflow/pkg/client/entities/ssh_client"
	"github.com/ovvesley/akoflow/pkg/client/services/install_kubernetes_local_service"
	"github.com/ovvesley/akoflow/pkg/client/services/ssh_connection_service"
)

const (
	commonBuildAddKubernetesRepoCommand      = "echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.33/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list"
	commonBuildAddKubernetesKeyringCommand   = "sudo mkdir -p /etc/apt/keyrings && curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.33/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg"
	commonBuildInstallPackagesCommand        = "sudo apt update && sudo apt install -y uidmap apt-transport-https ca-certificates curl gpg containerd kubelet kubeadm kubectl nfs-common"
	commonBuildHoldKubernetesPackagesCommand = "sudo apt-mark hold kubelet kubeadm kubectl && sudo sysctl -w net.ipv4.ip_forward=1"
	commonBuildConfigureContainerdCommand    = "sudo mkdir -p /etc/containerd && containerd config default | sudo tee /etc/containerd/config.toml > /dev/null && sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml && sudo systemctl restart containerd"
)

const (
	mainBuildInitializeKubernetes = "sudo kubeadm init --pod-network-cidr=192.168.0.0/16"
	mainBuildCopyKubeConfig       = "mkdir -p $HOME/.kube && sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config && sudo chown $(id -u):$(id -g) $HOME/.kube/config"
	mainBuildInstallCalico        = "kubectl apply -f https://docs.projectcalico.org/manifests/calico.yaml"

	// mainInstallAkoflowKubeAdmin = "kubectl apply -f https://raw.githubusercontent.com/UFFeScience/akoflow/refs/heads/main/pkg/server/resource/akoflow-kubeadmin.yaml"
)

type MainHostDTO struct {
	IPAddress string
	Token     string
	CertHash  string
}

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
		commonBuildAddKubernetesRepoCommand,
		commonBuildAddKubernetesKeyringCommand,
		commonBuildInstallPackagesCommand,
		commonBuildHoldKubernetesPackagesCommand,
		commonBuildConfigureContainerdCommand,
	})

	mainHost := i.sshConnectionService.GetMainNode()
	workers := i.sshConnectionService.GetWorkerNodes()

	mainHostDTO := i.handleKubernetesInitializationInMainHost(mainHost)
	i.handleKubernetesJoiningInWorkerNodes(workers, mainHostDTO)

	i.sshConnectionService.CloseConnections()
}

func (i *InstallAkoflowLocalService) handleKubernetesInitializationInMainHost(mainHost ssh_client_entity.SSHClient) MainHostDTO {
	output := i.sshConnectionService.ExecuteCommandsOnHost(mainHost, []string{
		mainBuildInitializeKubernetes,
		mainBuildCopyKubeConfig,
		mainBuildInstallCalico,
	})

	ipAddress := ""
	token := ""
	certHash := ""

	var re = regexp.MustCompile(`(?m)kubeadm\sjoin\s(.*?.)\s*--token\s(.*?)\\\s*?--discovery-token-ca-cert-hash\s(.*?)$`)
	matches := re.FindStringSubmatch(output)

	if len(matches) > 3 {
		ipAddress = matches[1]
		token = matches[2]
		certHash = matches[3]
	}

	mainHostDTO := MainHostDTO{
		IPAddress: ipAddress,
		Token:     token,
		CertHash:  certHash,
	}

	return mainHostDTO
}

func (i *InstallAkoflowLocalService) handleKubernetesJoiningInWorkerNodes(workers []ssh_client_entity.SSHClient, mainHostDTO MainHostDTO) {

	for _, worker := range workers {
		i.sshConnectionService.ExecuteCommandsOnHost(worker, []string{
			fmt.Sprintf("sudo kubeadm join %s --token %s --discovery-token-ca-cert-hash %s", mainHostDTO.IPAddress, mainHostDTO.Token, mainHostDTO.CertHash),
		})
	}

}
