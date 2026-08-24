package main

import (
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	"github.com/UFFeScience/akoflow/internal/provider"
	"github.com/UFFeScience/akoflow/internal/provider/kubernetes"
	"github.com/UFFeScience/akoflow/internal/provider/local"
	"github.com/UFFeScience/akoflow/internal/provider/registry"
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

func connectKubernetes(settings config.Settings) (kubernetes.API, error) {
	if settings.KubernetesAPIServer == "" && settings.KubernetesToken == "" {
		return nil, nil
	}
	client, err := kubernetes.NewClient(kubernetes.ClientConfig{
		Endpoint: settings.KubernetesAPIServer, Token: settings.KubernetesToken,
		CAFile:                settings.KubernetesCAFile,
		InsecureSkipTLSVerify: settings.KubernetesInsecureSkipTLS,
	})
	if err != nil {
		return nil, fmt.Errorf("configure Kubernetes API: %w", err)
	}
	return client, nil
}

func buildRuntimes(settings config.Settings, kubernetesAPI kubernetes.API, catalogs ...ports.EnvironmentCatalog) (ports.RuntimeResolver, error) {
	runtimes := registry.New()
	adapters := map[string]ports.RuntimeAdapter{
		"*":          simgrid.NewActivityRuntime(),
		"local":      local.New(),
		"kubernetes": kubernetes.New(kubernetesAPI, settings.DefaultNamespace),
		"slurm": slurm.NewWithConfig(provider.OSCommandExecutor{}, slurm.Config{
			ScriptDirectory: settings.SlurmScriptDirectory,
		}),
	}
	for runtimeID, adapter := range adapters {
		if err := runtimes.Register(runtimeID, adapter); err != nil {
			return nil, err
		}
	}
	if len(catalogs) > 0 && catalogs[0] != nil {
		return registry.NewCatalogResolver(runtimes, catalogs[0],
			kubernetes.ConnectionFactory{
				DefaultNamespace: settings.DefaultNamespace,
				Fallback: kubernetes.ClientConfig{Endpoint: settings.KubernetesAPIServer,
					Token: settings.KubernetesToken, CAFile: settings.KubernetesCAFile,
					InsecureSkipTLSVerify: settings.KubernetesInsecureSkipTLS},
			},
			slurm.ConnectionFactory{Executor: provider.OSCommandExecutor{},
				DefaultScriptDirectory: settings.SlurmScriptDirectory},
		), nil
	}
	return runtimes, nil
}
