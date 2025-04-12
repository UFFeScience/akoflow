package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/config/database_config"
	"github.com/ovvesley/akoflow/pkg/shared/utils/utils_read_file"
)

func GetVersion() string {

	versionEnv := os.Getenv("AKOFLOW_SERVER_VERSION")
	if versionEnv != "" {
		return versionEnv
	}
	return "dev-env"
}

func SetupEnv() {

	tokenFile, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	tokenEnv := os.Getenv("K8S_API_SERVER_TOKEN")

	loadDotEnv()

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
	println("AKOFLOW_SERVER_VERSION: ", os.Getenv("AKOFLOW_VERSION"))

}

func SetupDatabase() {

	cfg := database_config.Load()
	joinURL := cfg.JoinURL
	if joinURL == "" {
		fmt.Println("Iniciando rqlited como LÍDER")
		database_config.StartRaftLeader(cfg)
	} else {
		fmt.Println("Iniciando rqlited como WORKER (seguindo líder)")
		database_config.StartRaftFollower(cfg)
	}
}

func loadDotEnv() {

	file := utils_read_file.New().GetRootProjectPath() + "/.env"
	content := utils_read_file.New().ReadFile(file)

	splitedLine := strings.Split(content, "\n")

	for _, line := range splitedLine {
		if line != "" {
			env := strings.Split(line, "=")
			key := strings.TrimSpace(env[0])
			value := strings.Trim(strings.TrimSpace(env[1]), "'")
			os.Setenv(key, value)
		}
	}

}
