package main

import (
	"github.com/UFFeScience/akoflow/internal/api/handlers/workflow_engine_api_handler"
	appbuild "github.com/UFFeScience/akoflow/internal/application/build"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	appstorage "github.com/UFFeScience/akoflow/internal/application/storage"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	"github.com/UFFeScience/akoflow/internal/infrastructure/credentials/sshkey"
	planningplugin "github.com/UFFeScience/akoflow/internal/infrastructure/plugins/planning"
	"github.com/UFFeScience/akoflow/internal/provider"
	filesystem "github.com/UFFeScience/akoflow/internal/provider/storage/filesystem"
	s3 "github.com/UFFeScience/akoflow/internal/provider/storage/s3"
	sshfilesystem "github.com/UFFeScience/akoflow/internal/provider/storage/sshfilesystem"
)

func buildAPI(
	storage persistence,
	settings config.Settings,
	connections ports.ConnectionHealthMonitor,
	discovery ports.EnvironmentDiscovery,
	console ports.ConsoleCommands,
	terminal ports.InteractiveConsole,
	sshKeys *sshkey.Manager,
) (*workflow_engine_api_handler.Handler, error) {
	// Never expose the process filesystem as a storage browser. Local storage is
	// opt-in and must have a deliberately configured, bounded root.
	browsers := appstorage.Registry{domain.StorageSSH: sshfilesystem.New(storage.environments, provider.OSCommandExecutor{}), domain.StorageS3: s3.New(nil, nil), domain.StorageMinIO: s3.New(nil, nil)}
	if settings.LocalStorageRoot != "" {
		fs, err := filesystem.New(domain.StorageLocal, settings.LocalStorageRoot)
		if err != nil {
			return nil, err
		}
		browsers[domain.StorageLocal], browsers[domain.StoragePVC], browsers[domain.StorageNFS], browsers[domain.StorageLustre] = fs, fs, fs, fs
	}
	ssh := sshfilesystem.New(storage.environments, provider.OSCommandExecutor{})
	browsers[domain.StorageSSH] = ssh
	manager := appbuild.Manager{Root: settings.ArtifactStoreRoot, MaxBytes: settings.BuildContextMaxBytes, Catalog: storage.data}
	manager.Executor = appbuild.Executor{Catalog: storage.data, Contexts: manager, Runner: provider.OSCommandExecutor{}, Buildctl: settings.Buildctl, Apptainer: settings.Apptainer, ArtifactStoreRoot: settings.ArtifactStoreRoot}
	return workflow_engine_api_handler.New(workflow_engine_api_handler.Dependencies{
		Environments: storage.environments,
		Workflows:    storage.workflows,
		Plans:        storage.plans,
		Events:       storage.events,
		Validator:    planningplugin.NewValidator(),
		Executions:   storage.executions,
		Topologies:   storage.topologies,
		Scopes:       storage.topologies,
		Data:         storage.data,
		Resources:    storage.resources,
		Instance:     storage.instance,
		Connections:  connections,
		Discovery:    discovery,
		SSHKeys:      sshKeys,
		Audit:        storage.audit,
		Console:      console,
		Terminal:     terminal,
		Storage:      appstorage.NewBrowserCoordinator(storage.storage, browsers),
		Build:        manager,
	})
}
