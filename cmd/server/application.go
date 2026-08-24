package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/UFFeScience/akoflow/internal/api/handlers/workflow_engine_api_handler"
	"github.com/UFFeScience/akoflow/internal/api/httpserver"
	applicationconsole "github.com/UFFeScience/akoflow/internal/application/console"
	applicationenvironment "github.com/UFFeScience/akoflow/internal/application/environment"
	applicationexecution "github.com/UFFeScience/akoflow/internal/application/execution"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/controlplane/eventloop"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config/logger"
	"github.com/UFFeScience/akoflow/internal/infrastructure/credentials/sshkey"
	"github.com/UFFeScience/akoflow/internal/provider"
	"github.com/UFFeScience/akoflow/internal/provider/kubernetes"
	"github.com/UFFeScience/akoflow/internal/provider/local"
	"github.com/UFFeScience/akoflow/internal/provider/slurm"
)

type application struct {
	settings          config.Settings
	log               *logger.Logger
	database          *sql.DB
	api               *workflow_engine_api_handler.Handler
	eventLoop         *eventloop.Loop
	historyCleaner    *kubernetes.HistoryCleaner
	connectionMonitor *applicationenvironment.ConnectionMonitor
}

func newApplication(ctx context.Context, settings config.Settings, log *logger.Logger) (*application, error) {
	storage, err := openPersistence(ctx)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*application, error) {
		_ = storage.database.Close()
		return nil, err
	}
	kubernetesAPI, err := connectKubernetes(settings)
	if err != nil {
		return fail(err)
	}
	runtimes, err := buildRuntimes(settings, kubernetesAPI, storage.environments)
	if err != nil {
		return fail(err)
	}
	activities := applicationexecution.New(runtimes, storage.executions, storage.data)
	simulator, err := buildSimulator(settings)
	if err != nil {
		return fail(err)
	}
	loop, err := buildEventLoop(storage.events, storage.executions, storage.data, activities, simulator, settings.ArtifactStoreRoot)
	if err != nil {
		return fail(err)
	}
	connectionMonitor := applicationenvironment.NewConnectionMonitor(storage.environments,
		map[domain.ConnectionType]ports.ConnectionProber{
			domain.ConnectionKubernetes: kubernetes.NewConnectionProber(kubernetesAPI, settings.DefaultNamespace),
			domain.ConnectionSSH:        slurm.NewConnectionProber(provider.OSCommandExecutor{}),
			domain.ConnectionAgent:      slurm.NewConnectionProber(provider.OSCommandExecutor{}),
			domain.ConnectionLocal:      local.NewConnectionProber(),
		}, storage.audit)
	discovery := applicationenvironment.NewDiscoveryCoordinator(storage.environments, storage.resources,
		map[domain.ConnectionType]ports.ConnectionDiscoverer{
			domain.ConnectionKubernetes: kubernetes.NewDiscovery(kubernetes.ClientConfig{Endpoint: settings.KubernetesAPIServer, Token: settings.KubernetesToken, CAFile: settings.KubernetesCAFile, InsecureSkipTLSVerify: settings.KubernetesInsecureSkipTLS}),
			domain.ConnectionLocal:      local.NewDiscovery(),
			domain.ConnectionSSH:        slurm.NewDiscovery(provider.OSCommandExecutor{}),
			domain.ConnectionAgent:      slurm.NewDiscovery(provider.OSCommandExecutor{}),
		}, storage.audit)
	var consoleCommands ports.ConsoleCommands
	var terminal ports.InteractiveConsole
	if settings.ConsoleEnabled {
		controller := applicationconsole.NewCommandController(storage.environments, storage.resources, storage.console,
			slurm.ConsoleRunner{Executor: provider.OSCommandExecutor{}}, storage.audit)
		consoleCommands = controller
		terminal = applicationconsole.NewTerminalController(controller, terminalRunner{kubernetes: kubernetes.TerminalRunner{Fallback: kubernetes.ClientConfig{Endpoint: settings.KubernetesAPIServer, Token: settings.KubernetesToken, CAFile: settings.KubernetesCAFile, InsecureSkipTLSVerify: settings.KubernetesInsecureSkipTLS}}, slurm: slurm.TerminalRunner{}}, storage.audit, storage.console)
	}
	api, err := buildAPI(storage, settings, connectionMonitor, discovery, consoleCommands, terminal, sshkey.New(settings.SSHKeyDirectory))
	if err != nil {
		return fail(err)
	}
	cleaner, err := buildHistoryCleaner(settings, kubernetesAPI)
	if err != nil {
		return fail(err)
	}
	return &application{
		settings: settings, log: log, database: storage.database,
		api: api, eventLoop: loop, historyCleaner: cleaner,
		connectionMonitor: connectionMonitor,
	}, nil
}

func (a *application) Run(ctx context.Context) error {
	a.log.Info("Starting Akoflow Server")
	a.startEventLoop(ctx)
	a.startHistoryCleanup(ctx)
	go a.connectionMonitor.Run(ctx, a.settings.ConnectionCheckInterval)
	if err := httpserver.Serve(ctx, a.settings.HTTPAddress, a.api); err != nil {
		return fmt.Errorf("serve Akoflow API: %w", err)
	}
	return nil
}

func (a *application) Close() error {
	return a.database.Close()
}
