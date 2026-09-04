package main

import (
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	"github.com/UFFeScience/akoflow/internal/provider"
	cloudruntime "github.com/UFFeScience/akoflow/internal/provider/cloud/runtime"
	"github.com/UFFeScience/akoflow/internal/provider/kubernetes"
	"github.com/UFFeScience/akoflow/internal/provider/local"
	"github.com/UFFeScience/akoflow/internal/provider/registry"
	"github.com/UFFeScience/akoflow/internal/provider/remote"
	"github.com/UFFeScience/akoflow/internal/provider/simgrid"
	"github.com/UFFeScience/akoflow/internal/provider/slurm"
)

func buildSimulator(settings config.Settings) (ports.PlanExecutor, error) {
	switch strings.ToLower(strings.TrimSpace(settings.SimulationBackend)) {
	case "", "deterministic":
		return simgrid.NewSimulationExecutor(), nil
	case "simgrid":
		return simgrid.NewProcessExecutor(provider.OSCommandExecutor{}, simgrid.ProcessConfig{
			BinaryPath: settings.SimGridBinaryPath, Workspace: settings.SimGridWorkspace,
			MaxConcurrent: settings.SimGridMaxConcurrent, Timeout: settings.SimGridTimeout,
			ReferenceFLOPS: settings.SimGridReferenceFLOPS,
		})
	default:
		return nil, fmt.Errorf("unsupported simulation backend %q", settings.SimulationBackend)
	}
}

func buildRuntimes(
	settings config.Settings,
	catalog ports.EnvironmentCatalog,
	cloudStore ports.CloudConfigurationStore,
	cloudProvisioner ports.CloudProvisioner,
) (ports.RuntimeResolver, error) {
	runtimes := registry.New()
	adapters := map[string]ports.RuntimeAdapter{
		"*":          simgrid.NewActivityRuntime(),
		"local":      local.New(),
		"kubernetes": kubernetes.New(nil, settings.DefaultNamespace),
		"slurm": slurm.NewWithConfig(provider.OSCommandExecutor{}, slurm.Config{
			ScriptDirectory: settings.SlurmScriptDirectory,
		}),
	}
	for runtimeID, adapter := range adapters {
		if err := runtimes.Register(runtimeID, adapter); err != nil {
			return nil, err
		}
	}
	if catalog != nil {
		return registry.NewCatalogResolver(runtimes, catalog,
			kubernetes.ConnectionFactory{DefaultNamespace: settings.DefaultNamespace},
			remote.Factory{Executor: provider.OSCommandExecutor{}},
			cloudruntime.Factory{
				Store: cloudStore, Provisioner: cloudProvisioner,
				Executor: provider.OSCommandExecutor{},
			},
			slurm.ConnectionFactory{Executor: provider.OSCommandExecutor{},
				DefaultScriptDirectory: settings.SlurmScriptDirectory},
		), nil
	}
	return runtimes, nil
}
