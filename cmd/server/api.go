package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/api/handlers/workflow_engine_api_handler"
	appbuild "github.com/UFFeScience/akoflow/internal/application/build"
	applicationcloud "github.com/UFFeScience/akoflow/internal/application/cloudcatalog"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	appstorage "github.com/UFFeScience/akoflow/internal/application/storage"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	cloudcredential "github.com/UFFeScience/akoflow/internal/infrastructure/credentials/cloud"
	"github.com/UFFeScience/akoflow/internal/infrastructure/credentials/sshkey"
	"github.com/UFFeScience/akoflow/internal/infrastructure/credentials/token"
	"github.com/UFFeScience/akoflow/internal/infrastructure/database"
	databaseprovenance "github.com/UFFeScience/akoflow/internal/infrastructure/database/provenance"
	"github.com/UFFeScience/akoflow/internal/infrastructure/instancearchive"
	planningplugin "github.com/UFFeScience/akoflow/internal/infrastructure/plugins/planning"
	"github.com/UFFeScience/akoflow/internal/provider"
	gcpcloud "github.com/UFFeScience/akoflow/internal/provider/cloud/gcp"
	"github.com/UFFeScience/akoflow/internal/provider/kubernetes"
	"github.com/UFFeScience/akoflow/internal/provider/local"
	"github.com/UFFeScience/akoflow/internal/provider/slurm"
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
	planning workflow_engine_api_handler.PlanningOrchestrator,
) (*workflow_engine_api_handler.Handler, error) {
	cloudCredentials := cloudcredential.New(settings.CloudCredentialDirectory)
	cloudCatalog := applicationcloud.New(storage.environments, cloudCredentials, gcpcloud.New(nil))
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
	archives, err := instancearchive.New(
		storage.database,
		instancearchive.ResolveDatabasePath(),
		instancearchive.DefaultRoot(),
		settings.ArtifactStoreRoot,
		config.GetVersion(),
	)
	if err != nil {
		return nil, err
	}
	var restart func()
	if strings.EqualFold(strings.TrimSpace(os.Getenv("AKOFLOW_RESTART_ON_INSTANCE_SWITCH")), "true") {
		restart = func() {
			time.Sleep(300 * time.Millisecond)
			os.Exit(0)
		}
	}
	return workflow_engine_api_handler.New(workflow_engine_api_handler.Dependencies{
		Environments:     storage.environments,
		Workflows:        storage.workflows,
		Plans:            storage.plans,
		Events:           storage.events,
		Validator:        planningplugin.NewValidator(),
		Executions:       storage.executions,
		Topologies:       storage.topologies,
		Scopes:           storage.topologies,
		Data:             storage.data,
		Resources:        storage.resources,
		Instance:         storage.instance,
		Connections:      connections,
		Discovery:        discovery,
		SSHKeys:          sshKeys,
		KubernetesTokens: token.New(settings.KubernetesTokenDirectory),
		Audit:            storage.audit,
		Console:          console,
		Terminal:         terminal,
		Storage:          appstorage.NewBrowserCoordinator(storage.storage, browsers),
		Build:            manager,
		Planning:         planning,
		PlanningStore:    storage.plans,
		Provenance:       databaseprovenance.New(storage.database),
		Cloud:            storage.cloud,
		CloudCatalog:     cloudCatalog,
		CloudCredentials: cloudCredentials,
		InstanceArchive:  archives,
		ReadOnly:         storage.readOnly,
		Restart:          restart,
		FactoryReset: func(ctx context.Context) error {
			if err := database.Reset(ctx, storage.database); err != nil {
				return err
			}
			// SSH keys can be an operator-provided read-only volume. They are not
			// owned by the database and must never make a factory reset fail.
			if err := os.RemoveAll(settings.KubernetesTokenDirectory); err != nil {
				return fmt.Errorf("remove managed Kubernetes credentials: %w", err)
			}
			return nil
		},
		ConnectionTest: func(ctx context.Context, connection domain.EnvironmentConnection) ports.ConnectionHealth {
			switch connection.Type {
			case domain.ConnectionKubernetes:
				return kubernetes.NewConnectionProber(settings.DefaultNamespace).Probe(ctx, connection)
			case domain.ConnectionSSH, domain.ConnectionAgent:
				return slurm.NewConnectionProber(provider.OSCommandExecutor{}).Probe(ctx, connection)
			case domain.ConnectionLocal:
				return local.NewConnectionProber().Probe(ctx, connection)
			default:
				return ports.ConnectionHealth{Message: "unsupported connection type"}
			}
		},
	})
}
