package runtimes

import (
	"fmt"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
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
	GetLogs(workflow workflow_entity.Workflow, workflowActivity workflow_activity_entity.WorkflowActivities) string

	GetStatus(workflowID int, activityID int) string

	VerifyActivitiesWasFinished(workflow workflow_entity.Workflow) bool

	HealthCheck() bool
}

func normalizeRuntime(runtime string) string {
	// if runtime start with k8s or k8s://, set it to k8s

	if strings.HasPrefix(runtime, "k8s") {
		return RUNTIME_K8S
	}

	if strings.HasPrefix(runtime, "sdumont") {
		return RUNTIME_SINGULARITY_SDUMONT
	}

	return runtime

}

func GetRuntimeInstance(runtimeName string) IRuntime {

	runtime := normalizeRuntime(runtimeName)

	modeMap := map[string]IRuntime{
		RUNTIME_DOCKER:              docker_runtime.NewDockerRuntime(),
		RUNTIME_K8S:                 kubernetes_runtime.NewKubernetesRuntime().SetRuntimeName(runtimeName),
		RUNTIME_SINGULARITY:         singularity_runtime.NewSingularityRuntime(),
		RUNTIME_SINGULARITY_SDUMONT: sdumont_runtime.NewSdumontRuntime().SetRuntimeName(runtimeName),
	}
	if modeMap[runtime] == nil {
		config.App().Logger.Error(fmt.Sprintf("Runtime not found: %s", runtimeName))
	}
	return modeMap[runtime]
}
