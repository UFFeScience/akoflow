package config

import (
	"os"
	"strings"

	"github.com/ovvesley/akoflow/pkg/shared/utils/utils_read_file"
)

const PORT_SERVER = ":8080"

func GetVersion() string {

	versionEnv := os.Getenv("AKOFLOW_SERVER_VERSION")
	if versionEnv != "" {
		return versionEnv
	}
	return "dev-env"
}

// depreacated to be removed in the future. This item exists to garanted that read service account key file
func SetupEnv() {

	tokenFile, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	tokenEnv := os.Getenv("K8S_API_SERVER_TOKEN")

	loadDotEnv()

	hostEnvByKube := os.Getenv("KUBERNETES_SERVICE_HOST")

	tokenData := ""

	if tokenEnv == "" && tokenFile != nil {
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

	if tokenData != "" {
		os.Setenv("K8S_API_SERVER_TOKEN", tokenData)
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
