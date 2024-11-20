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
