package main

import (
	"flag"
	"os"
	"strconv"

	"github.com/ovvesley/akoflow/pkg/client/services/dispatch_to_server_run_workflow_service"
)

func main() {
	host := flag.String("host", "localhost", "host")
	port := flag.String("port", "8080", "port")
	fileYaml := flag.String("file", "", "file")

	flag.Parse()

	if !validateFile(*fileYaml) {
		panic("Invalid file")
	}

	if !validateHost(*host) {
		panic("Invalid host")
	}

	if !validatePort(*port) {
		panic("Invalid port")
	}

	dispatchToServerRunWorkflowService := dispatch_to_server_run_workflow_service.New()

	dispatchToServerRunWorkflowService.
		SetHost(*host).
		SetPort(*port).
		SetFile(*fileYaml).
		Run()

}

func validateFile(file string) bool {
	if file == "" {
		return false
	}

	if _, err := os.Stat(file); os.IsNotExist(err) {
		return false
	}

	return true
}

func validateHost(host string) bool {
	return host != ""
}

func validatePort(port string) bool {
	if port == "" {
		return false
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return false
	}

	return portNumber > 0 && portNumber < 65535

}
