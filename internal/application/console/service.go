package console

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainaudit "github.com/UFFeScience/akoflow/internal/domain/audit"
	domainconsole "github.com/UFFeScience/akoflow/internal/domain/console"
	"github.com/UFFeScience/akoflow/internal/provider"
)

type CommandController struct {
	catalog   ports.EnvironmentCatalog
	resources ports.ResourceInventory
	store     ports.ConsoleCommandStore
	runner    ports.ConsoleCommandRunner
	audit     ports.AuditStore
}

var _ ports.ConsoleCommands = (*CommandController)(nil)

func NewCommandController(catalog ports.EnvironmentCatalog, resources ports.ResourceInventory, store ports.ConsoleCommandStore, runner ports.ConsoleCommandRunner, audit ports.AuditStore) *CommandController {
	return &CommandController{catalog: catalog, resources: resources, store: store, runner: runner, audit: audit}
}

func (s *CommandController) ExecuteCommand(ctx context.Context, request domainconsole.Request) (domainconsole.Command, error) {
	if strings.TrimSpace(request.ResourceID) == "" || strings.TrimSpace(request.Command) == "" {
		return domainconsole.Command{}, fmt.Errorf("resourceId and command are required")
	}
	resource, err := s.resources.FindByID(ctx, request.ResourceID)
	if err != nil || resource == nil {
		if err == nil {
			err = fmt.Errorf("resource %q was not found", request.ResourceID)
		}
		return domainconsole.Command{}, err
	}
	runtime, connection, environmentID, err := s.resolveTarget(ctx, *resource)
	if err != nil {
		return domainconsole.Command{}, err
	}
	timeout := request.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 3600 {
		return domainconsole.Command{}, fmt.Errorf("timeoutSeconds cannot exceed 3600")
	}
	now := time.Now().UTC()
	command := domainconsole.Command{ID: provider.NewID("console-command"), ResourceID: resource.ID,
		RuntimeID: runtime.ID, ConnectionID: connection.ID, ActorID: request.ActorID, Command: request.Command,
		WorkingDirectory: request.WorkingDirectory, Environment: request.Environment, CPUCores: request.CPUCores,
		MemoryBytes: request.MemoryBytes, TimeoutSeconds: timeout, Status: domainconsole.CommandRunning,
		CreatedAt: now, StartedAt: now}
	if err := s.store.SaveConsoleCommand(ctx, command); err != nil {
		return domainconsole.Command{}, err
	}
	s.auditEvent(ctx, command, environmentID, "console.command.started", domainaudit.OutcomeStarted, "Remote command started")
	runContext, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	stdout, stderr, exitCode, externalID, runErr := s.runner.RunConsoleCommand(runContext, connection, *resource, command)
	finished := time.Now().UTC()
	command.Stdout, command.Stderr, command.ExitCode, command.ExternalID, command.FinishedAt = stdout, stderr, &exitCode, externalID, &finished
	if runErr != nil {
		command.Status, command.Failure = domainconsole.CommandFailed, runErr.Error()
		s.auditEvent(ctx, command, environmentID, "console.command.failed", domainaudit.OutcomeFailed, command.Failure)
	} else {
		command.Status = domainconsole.CommandCompleted
		s.auditEvent(ctx, command, environmentID, "console.command.completed", domainaudit.OutcomeSucceeded, "Remote command completed")
	}
	if err := s.store.SaveConsoleCommand(ctx, command); err != nil {
		return domainconsole.Command{}, err
	}
	return command, nil
}

func (s *CommandController) ListCommands(ctx context.Context, limit int) ([]domainconsole.Command, error) {
	return s.store.ListConsoleCommands(ctx, limit)
}

func (s *CommandController) resolveTarget(ctx context.Context, resource domain.Resource) (domain.EnvironmentRuntime, domain.EnvironmentConnection, string, error) {
	definitions, err := s.catalog.List(ctx)
	if err != nil {
		return domain.EnvironmentRuntime{}, domain.EnvironmentConnection{}, "", err
	}
	for _, definition := range definitions {
		if definition.Version.ID != resource.EnvironmentVersionID {
			continue
		}
		for _, binding := range definition.RuntimeBindings {
			if binding.ResourceID != resource.ID || !binding.Enabled {
				continue
			}
			for _, runtime := range definition.Runtimes {
				if runtime.ID != binding.RuntimeID {
					continue
				}
				connectionID, _ := runtime.Configuration["connectionId"].(string)
				for _, connection := range definition.Connections {
					if connection.ID == connectionID {
						return runtime, connection, definition.Environment.ID, nil
					}
				}
			}
		}
	}
	return domain.EnvironmentRuntime{}, domain.EnvironmentConnection{}, "", fmt.Errorf("resource %q has no connected runtime", resource.ID)
}

func (s *CommandController) auditEvent(ctx context.Context, command domainconsole.Command, environmentID, eventType string, outcome domainaudit.Outcome, summary string) {
	if s.audit == nil {
		return
	}
	_ = s.audit.RecordAuditEvent(ctx, domainaudit.Event{ID: provider.NewID("audit"), EventType: eventType,
		ActorID: command.ActorID, ActorType: "user", EnvironmentID: environmentID, ResourceID: command.ResourceID,
		ConnectionID: command.ConnectionID, RuntimeID: command.RuntimeID, SessionID: command.ID, ExternalID: command.ExternalID,
		Outcome: outcome, Summary: summary, Metadata: map[string]any{"timeoutSeconds": command.TimeoutSeconds,
			"cpuCores": command.CPUCores, "memoryBytes": command.MemoryBytes, "exitCode": command.ExitCode}, OccurredAt: time.Now().UTC()})
}
