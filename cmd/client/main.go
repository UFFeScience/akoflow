package main

import (
	"github.com/ovvesley/scientific-workflow-k8s/pkg/server/connector"
)

func main() {

	URI := "http://workflow-controller-service:8080"

	response := connector.New(URI).Connect()
	println("Client is UP")

	println(response)

}
