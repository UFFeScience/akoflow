package config

import "os"

const PORT_SERVER = ":8080"

func SetupEnv() {

	tokenFile, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	tokenEnv := os.Getenv("K8S_API_SERVER_TOKEN")

	hostEnvByKube := os.Getenv("KUBERNETES_SERVICE_HOST")

	tokenData := ""

	if tokenEnv == "" {
		if err != nil {
			println("Error reading token file", err)
			panic(err)
		} else {
			tokenData = string(tokenFile)
		}
	} else {
		tokenData = tokenEnv
	}

	if hostEnvByKube != "" {
		os.Setenv("K8S_API_SERVER_HOST", hostEnvByKube)
	}

	os.Setenv("K8S_API_SERVER_TOKEN", tokenData)
	println("K8S_API_SERVER_HOST: ", os.Getenv("K8S_API_SERVER_HOST"))
	println("K8S_API_SERVER_TOKEN: ", os.Getenv("K8S_API_SERVER_TOKEN"))

}
