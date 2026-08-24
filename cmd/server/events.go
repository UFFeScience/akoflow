package main

import (
	"fmt"
	"os"
	"time"

	applicationexecution "github.com/UFFeScience/akoflow/internal/application/execution"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	applicationtransfer "github.com/UFFeScience/akoflow/internal/application/transfer"
	"github.com/UFFeScience/akoflow/internal/controlplane/eventloop"
	controlexecution "github.com/UFFeScience/akoflow/internal/controlplane/execution"
	domainevents "github.com/UFFeScience/akoflow/internal/domain/events"
	infratransfer "github.com/UFFeScience/akoflow/internal/infrastructure/transfer"
)

func buildEventLoop(
	events ports.QueueStore,
	executions ports.ExecutionStore,
	data ports.DataCatalog,
	activities *applicationexecution.Controller,
	simulator ports.PlanExecutor,
	artifactStoreRoot string,
) (*eventloop.Loop, error) {
	dispatcher := eventloop.NewDispatcher()
	if err := registerExecutionHandlers(dispatcher, executions, data, activities, simulator, artifactStoreRoot); err != nil {
		return nil, err
	}
	for _, eventType := range domainEventTypes() {
		if err := dispatcher.Register(eventType, eventloop.DomainEventHandler{}); err != nil {
			return nil, err
		}
	}
	owner, _ := os.Hostname()
	config := eventloop.DefaultConfig(fmt.Sprintf("%s-%d", owner, os.Getpid()))
	config.PollInterval = 500 * time.Millisecond
	return eventloop.New(events, dispatcher, config)
}

func registerExecutionHandlers(
	dispatcher *eventloop.Dispatcher,
	executions ports.ExecutionStore,
	data ports.DataCatalog,
	activities *applicationexecution.Controller,
	simulator ports.PlanExecutor,
	artifactStoreRoot string,
) error {
	if err := dispatcher.Register(eventloop.EventActivityExecutionRequested,
		eventloop.NewActivityExecutionHandler(activities)); err != nil {
		return err
	}
	preparer := applicationtransfer.Coordinator{Catalog: data, Materializer: applicationtransfer.Materializer{Connectors: []ports.TransferConnector{
		infratransfer.ArtifactStore{Root: artifactStoreRoot}, infratransfer.LocalFilesystem{}, infratransfer.RsyncSSH{}, infratransfer.HTTPDownload{}, infratransfer.S3Compatible{}, infratransfer.GCS{},
	}}}
	supervisor, err := controlexecution.New(executions, activities,
		simulator, controlexecution.Config{PollInterval: time.Second, MaxParallel: 8, Preparer: preparer})
	if err != nil {
		return err
	}
	return dispatcher.Register(eventloop.EventExecutionRunRequested,
		eventloop.NewExecutionRunHandler(supervisor))
}

func domainEventTypes() []string {
	return []string{
		domainevents.ExecutionStarted, domainevents.ExecutionCompleted, domainevents.ExecutionFailed,
		domainevents.ActivityStarted, domainevents.ActivityCompleted, domainevents.ActivityFailed,
	}
}
