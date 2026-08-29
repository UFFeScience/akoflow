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
	applicationplanning "github.com/UFFeScience/akoflow/internal/application/planning"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/controlplane/eventloop"
	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config"
	"github.com/UFFeScience/akoflow/internal/infrastructure/config/logger"
	"github.com/UFFeScience/akoflow/internal/infrastructure/credentials/sshkey"
	planningplugin "github.com/UFFeScience/akoflow/internal/infrastructure/plugins/planning"
	"github.com/UFFeScience/akoflow/internal/planning/algorithms"
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
	runtimes, err := buildRuntimes(settings, storage.environments)
	if err != nil {
		return fail(err)
	}
	activities := applicationexecution.New(runtimes, storage.executions, storage.data)
	simulator, err := buildSimulator(settings)
	if err != nil {
		return fail(err)
	}
	registry, err := algorithms.NewRegistry(algorithms.HEFT{}, algorithms.NewPRISMTime(), algorithms.NewPRISMCost())
	if err != nil {
		return fail(err)
	}
	planningService := &applicationplanning.Coordinator{
		Store: storage.plans, Plans: storage.plans, Workflows: storage.workflows,
		Environments: storage.environments, Resources: storage.resources,
		Scopes: storage.topologies, Topologies: storage.topologies,
		Validator: planningplugin.NewValidator(), Registry: registry, Events: storage.events,
	}
	loop, err := buildEventLoop(storage.events, storage.executions, storage.data, storage.instance, storage.environments, activities, simulator, settings.ArtifactStoreRoot, planningService)
	if err != nil {
		return fail(err)
	}
	connectionMonitor := applicationenvironment.NewConnectionMonitor(storage.environments,
		map[domain.ConnectionType]ports.ConnectionProber{
			domain.ConnectionKubernetes: kubernetes.NewConnectionProber(settings.DefaultNamespace),
			domain.ConnectionSSH:        slurm.NewConnectionProber(provider.OSCommandExecutor{}),
			domain.ConnectionAgent:      slurm.NewConnectionProber(provider.OSCommandExecutor{}),
			domain.ConnectionLocal:      local.NewConnectionProber(),
		}, storage.audit)
	discovery := applicationenvironment.NewDiscoveryCoordinator(storage.environments, storage.resources,
		map[domain.ConnectionType]ports.ConnectionDiscoverer{
			domain.ConnectionKubernetes: kubernetes.NewDiscovery(),
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
		terminal = applicationconsole.NewTerminalController(controller, terminalRunner{kubernetes: kubernetes.TerminalRunner{}, slurm: slurm.TerminalRunner{}}, storage.audit, storage.console)
	}
	api, err := buildAPI(storage, settings, connectionMonitor, discovery, consoleCommands, terminal, sshkey.New(settings.SSHKeyDirectory), planningService)
	if err != nil {
		return fail(err)
	}
	return &application{
		settings: settings, log: log, database: storage.database,
		api: api, eventLoop: loop,
		connectionMonitor: connectionMonitor,
	}, nil
}

func (a *application) Run(ctx context.Context) error {
	a.log.Info("Starting Akoflow Server")
	a.startEventLoop(ctx)
	go a.connectionMonitor.Run(ctx, a.settings.ConnectionCheckInterval)
	if err := httpserver.Serve(ctx, a.settings.HTTPAddress, a.api); err != nil {
		return fmt.Errorf("serve Akoflow API: %w", err)
	}
	return nil
}

func (a *application) Close() error {
	return a.database.Close()
}
