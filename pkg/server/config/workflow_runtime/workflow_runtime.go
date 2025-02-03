package workflow_runtime

import (
	"github.com/ovvesley/akoflow/pkg/server/runtimes"
)

type WorkflowRuntime struct {
}

func New() WorkflowRuntime {
	return WorkflowRuntime{}
}

func (w *WorkflowRuntime) GetRuntimeInstance(runtime string) runtimes.IRuntime {

	modeMap := map[string]runtimes.IRuntime{
		runtimes.RUNTIME_DOCKER:              runtimes.NewDockerRuntime(),
		runtimes.RUNTIME_K8S:                 runtimes.NewK8sRuntime(),
		runtimes.RUNTIME_SINGULARITY:         runtimes.NewSingularityRuntime(),
		runtimes.RUNTIME_SINGULARITY_SDUMONT: runtimes.NewSingularitySdumontRuntime(),
	}

	return modeMap[runtime]
}
