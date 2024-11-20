package install_cli_service

import (
	"flag"
	"os"

	"github.com/ovvesley/akoflow/pkg/client/services/install_kubernetes_local_service"
)

type InstallCliService struct {
}

func New() *InstallCliService {
	return &InstallCliService{}
}

func (i *InstallCliService) Run() {
	// Install the CLI service

	// akoflow install --enviroment=local

	instance := flag.String("instance", "local", "instance")
	flag.CommandLine.Parse(os.Args[2:])

	println("Installing CLI service on", *instance)

	install_kubernetes_local_service := install_kubernetes_local_service.New()

	install_kubernetes_local_service.Install()

	// Install the CLI service

}
