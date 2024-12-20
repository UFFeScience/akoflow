package install_kubernetes_local_service

import (
	"os/exec"

	"github.com/ovvesley/akoflow/pkg/shared/utils/utils_exec_command"
)

type InstallKubernetesLocalService struct {
}

func New() *InstallKubernetesLocalService {
	return &InstallKubernetesLocalService{}
}

func (i *InstallKubernetesLocalService) Install() {

	if !i.verifyDocker() {
		i.installDocker()
	}

	if !i.verifyDocker() {
		panic("Docker installation failed")
	}

}

func (i *InstallKubernetesLocalService) verifyDocker() bool {
	if _, err := exec.LookPath("docker"); err != nil {
		return false
	}
	return true
}

func (i *InstallKubernetesLocalService) installDocker() error {
	utils_exec_command.New().RunCommand("curl", "-v", "-fsSL", "https://get.docker.com", "-o", "get-docker.sh")
	if _, err := exec.LookPath("sudo"); err == nil {
		utils_exec_command.New().RunCommand("sudo", "sh", "get-docker.sh")
	} else {
		utils_exec_command.New().RunCommand("sh", "get-docker.sh")
	}
	return nil
}

func (i *InstallKubernetesLocalService) verifyKubectl() bool {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return false
	}
	return true
}

// sudo mkdir -p /etc/containerd
// containerd config default | sudo tee /etc/containerd/config.toml
// sudo sed -i 's#sandbox_image = .*#sandbox_image = "registry.k8s.io/pause:3.9"#' /etc/containerd/config.toml

// # Reiniciar os serviços
// sudo systemctl restart containerd
// sudo systemctl restart docker

// sudo apt-get update
// sudo apt-get install -y apt-transport-https ca-certificates curl gpg

// curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.31/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg

// echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.31/deb/ /' | sudo tee /etc/apt/sources.list.d/kubernetes.list

// sudo apt-get update
// sudo apt-get install -y kubelet kubeadm kubectl
// sudo apt-mark hold kubelet kubeadm kubectl

// sudo systemctl enable --now kubelet
