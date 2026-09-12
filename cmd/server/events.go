package main

import (
	"context"
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
	instance ports.InstanceStore,
	connections ports.ConnectionStore,
	activities *applicationexecution.Controller,
	simulator ports.PlanExecutor,
	artifactStoreRoot string,
	planning eventloop.PlanningRunner,
	expansions eventloop.WorkflowExpansionRunner,
	cloud interface {
		ports.CloudConfigurationStore
		ports.CloudOperationStore
	},
	cloudProvisioner ports.CloudProvisioner,
) (*eventloop.Loop, error) {
	dispatcher := eventloop.NewDispatcher()
	if err := registerExecutionHandlers(
		dispatcher, events, executions, data, instance, connections, activities, simulator,
		artifactStoreRoot, cloud, cloudProvisioner,
	); err != nil {
		return nil, err
	}
	if err := dispatcher.Register(eventloop.EventPlanningSessionRequested, eventloop.NewPlanningSessionHandler(planning)); err != nil {
		return nil, err
	}
	if err := dispatcher.Register(eventloop.EventWorkflowExpansionRequested, eventloop.NewWorkflowExpansionHandler(expansions)); err != nil {
		return nil, err
	}
	cloudHandler := eventloop.NewCloudOperationHandler(cloud, cloudProvisioner)
	for _, eventType := range eventloop.CloudOperationEventTypes() {
		if err := dispatcher.Register(eventType, cloudHandler); err != nil {
			return nil, err
		}
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
	events ports.QueueStore,
	executions ports.ExecutionStore,
	data ports.DataCatalog,
	instance ports.InstanceStore,
	connections ports.ConnectionStore,
	activities *applicationexecution.Controller,
	simulator ports.PlanExecutor,
	artifactStoreRoot string,
	cloud interface {
		ports.CloudConfigurationStore
		ports.CloudOperationStore
	},
	cloudProvisioner ports.CloudProvisioner,
) error {
	if err := dispatcher.Register(eventloop.EventActivityExecutionRequested,
		eventloop.NewActivityExecutionHandler(activities)); err != nil {
		return err
	}
	bufferSize := infratransfer.InstanceBufferSize(instance)
	resolver := infratransfer.EnvironmentEndpointResolver{
		Connections: connections, Cloud: cloud, Provisioner: cloudProvisioner,
	}
	preparer := applicationtransfer.Coordinator{Catalog: data, Materializer: applicationtransfer.Materializer{Resolver: resolver, Progress: data, ChunkSize: func(ctx context.Context) int64 { return int64(bufferSize(ctx)) }, Connectors: []ports.TransferConnector{
		infratransfer.ArtifactStore{Root: artifactStoreRoot}, infratransfer.LocalFilesystem{BufferSize: bufferSize}, infratransfer.RsyncSSH{BufferSize: bufferSize}, &infratransfer.KubernetesExec{BufferSize: bufferSize}, infratransfer.HTTPDownload{}, infratransfer.S3Compatible{BufferSize: bufferSize}, infratransfer.GCS{},
	}}}
	supervisor, err := controlexecution.New(
		executions, activities, simulator,
		controlexecution.Config{
			PollInterval: time.Second, MaxParallel: 8, Preparer: preparer,
			Data: data, Cloud: cloudProvisioner, CloudStore: cloud,
			CloudAllocator: controlexecution.QueuedCloudAllocator{Cloud: cloud, Operations: cloud, Queue: events, PollInterval: 500 * time.Millisecond},
		},
	)
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
