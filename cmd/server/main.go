package main

import (
	"os"

	"github.com/ovvesley/scik8sflow/pkg/server/httpserver"
	"github.com/ovvesley/scik8sflow/pkg/server/monitor"
	"github.com/ovvesley/scik8sflow/pkg/server/orchestrator"
	"github.com/ovvesley/scik8sflow/pkg/server/worker"
)

func main() {

	// setup env
	setupEnv()

	go worker.StartWorker()
	go orchestrator.StartOrchestrator()
	go monitor.StartMonitor()

	httpserver.StartServer()

}

func setupEnv() {
	os.Setenv("K8S_API_SERVER_HOST", os.Getenv("KUBERNETES_SERVICE_HOST"))
	os.Setenv("K8S_API_SERVER_PORT", os.Getenv("KUBERNETES_SERVICE_PORT"))

	token, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		print("Service account token not detected in the pod. Please make sure the deployment definition has the service account token mounted.")
		panic(err)
	}

	os.Setenv("K8S_API_SERVER_TOKEN", string(token))
	println("K8S_API_SERVER_HOST: ", os.Getenv("K8S_API_SERVER_HOST"))
	println("K8S_API_SERVER_PORT: ", os.Getenv("K8S_API_SERVER_PORT"))
	println("K8S_API_SERVER_TOKEN: ", os.Getenv("K8S_API_SERVER_TOKEN"))

}
