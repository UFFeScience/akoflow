package runtimes

import (
	"fmt"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/runtimes/docker_runtime"
	"github.com/ovvesley/akoflow/pkg/server/runtimes/kubernetes_runtime"
	"github.com/ovvesley/akoflow/pkg/server/runtimes/sdumont_runtime"
	"github.com/ovvesley/akoflow/pkg/server/runtimes/singularity_runtime"
)

const RUNTIME_K8S = "k8s"
const RUNTIME_DOCKER = "docker"
const RUNTIME_SINGULARITY = "singularity"
const RUNTIME_SINGULARITY_SDUMONT = "sdumont"

type IRuntime interface {
	StartConnection() error
	StopConnection() error

	ApplyJob(workflowID int, activityID int) bool
	DeleteJob(workflowID int, activityID int) bool

	GetMetrics(workflowID int, activityID int) string
	GetLogs(workflowID int, activityID int) string

	GetStatus(workflowID int, activityID int) string
}

func GetRuntimeInstance(runtime string) IRuntime {

	modeMap := map[string]IRuntime{
		RUNTIME_DOCKER:              docker_runtime.NewDockerRuntime(),
		RUNTIME_K8S:                 kubernetes_runtime.NewKubernetesRuntime(),
		RUNTIME_SINGULARITY:         singularity_runtime.NewSingularityRuntime(),
		RUNTIME_SINGULARITY_SDUMONT: sdumont_runtime.NewSdumontRuntime(),
	}
	if modeMap[runtime] == nil {
		config.App().Logger.Error(fmt.Sprintf("Runtime not found: %s", runtime))
	}
	return modeMap[runtime]
}
