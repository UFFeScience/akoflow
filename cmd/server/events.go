package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	applicationbuild "github.com/UFFeScience/akoflow/internal/application/build"
	applicationexecution "github.com/UFFeScience/akoflow/internal/application/execution"
	"github.com/UFFeScience/akoflow/internal/application/ports"
	applicationtransfer "github.com/UFFeScience/akoflow/internal/application/transfer"
	"github.com/UFFeScience/akoflow/internal/controlplane/eventloop"
	controlexecution "github.com/UFFeScience/akoflow/internal/controlplane/execution"
	"github.com/UFFeScience/akoflow/internal/domain"
	domainevents "github.com/UFFeScience/akoflow/internal/domain/events"
	infratransfer "github.com/UFFeScience/akoflow/internal/infrastructure/transfer"
	"github.com/UFFeScience/akoflow/internal/provider"
	localprovider "github.com/UFFeScience/akoflow/internal/provider/local"
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
	cloud interface {
		ports.CloudConfigurationStore
		ports.CloudOperationStore
	},
	cloudProvisioner ports.CloudProvisioner,
	prepare func(context.Context, *ports.ExecutionRequest) error,
	prepareSIF func(context.Context, string, domain.Activity, domain.Resource) (domain.PreparationRequirement, error),
) (*eventloop.Loop, error) {
	dispatcher := eventloop.NewDispatcher()
	if err := registerExecutionHandlers(
		dispatcher, events, executions, data, instance, connections, activities, simulator,
		artifactStoreRoot, cloud, cloudProvisioner, prepare, prepareSIF,
	); err != nil {
		return nil, err
	}
	if err := dispatcher.Register(eventloop.EventPlanningSessionRequested, eventloop.NewPlanningSessionHandler(planning)); err != nil {
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
	prepare func(context.Context, *ports.ExecutionRequest) error,
	prepareSIF func(context.Context, string, domain.Activity, domain.Resource) (domain.PreparationRequirement, error),
) error {
	if err := dispatcher.Register(eventloop.EventActivityExecutionRequested,
		eventloop.NewActivityExecutionHandler(activities)); err != nil {
		return err
	}
	bufferSize := infratransfer.InstanceBufferSize(instance)
	resolver := infratransfer.EnvironmentEndpointResolver{
		Connections: connections, Cloud: cloud, Provisioner: cloudProvisioner,
	}
	verifiedArtifacts := &applicationtransfer.VerifiedArtifactCache{}
	reproducer := &applicationbuild.Reproducer{Manager: applicationbuild.Manager{Root: artifactStoreRoot, Catalog: data}, Catalog: data, Runner: provider.OSCommandExecutor{}, Buildctl: os.Getenv("AKOFLOW_BUILDCTL")}
	kubernetesTransfer := &infratransfer.KubernetesExec{BufferSize: bufferSize}
	parallelism, err := strconv.Atoi(os.Getenv("AKOFLOW_WORKSPACE_TRANSFER_PARALLELISM"))
	if err != nil || parallelism <= 0 {
		parallelism = 8
	}
	preparer := applicationtransfer.Coordinator{Catalog: data, Reproducer: reproducer, WorkspaceSyncer: infratransfer.WorkspaceRsync{Resolver: resolver, Parallelism: parallelism}, Materializer: applicationtransfer.Materializer{Resolver: resolver, Progress: data, VerifiedArtifacts: verifiedArtifacts, Connectors: []ports.TransferConnector{
		infratransfer.ArtifactStore{Root: artifactStoreRoot}, infratransfer.LocalFilesystem{BufferSize: bufferSize}, infratransfer.RsyncSSH{BufferSize: bufferSize}, kubernetesTransfer, infratransfer.HTTPDownload{}, infratransfer.S3Compatible{BufferSize: bufferSize}, infratransfer.GCS{},
	}}}
	capacity, err := sharedLocalCapacity()
	if err != nil {
		return err
	}
	var reconcileCapacity func(context.Context) error
	if store, ok := executions.(ports.LocalAdmissionStore); ok && capacity.CPU > 0 {
		if err := store.RecoverLocalPreparations(context.Background()); err != nil {
			return fmt.Errorf("recover local preparation reservations: %w", err)
		}
		reconcileCapacity = func(ctx context.Context) error {
			return reconcileLocalReservations(ctx, store, provider.OSCommandExecutor{})
		}
	}
	supervisor, err := controlexecution.New(
		executions, activities, simulator,
		controlexecution.Config{
			PollInterval: time.Second, Preparer: preparer, PrepareRun: prepare, PrepareSIF: prepareSIF, SharedLocalCapacity: capacity, ReconcileLocalCapacity: reconcileCapacity,
			Data: data, Cloud: cloudProvisioner, CloudStore: cloud,
			Workspaces:     infratransfer.ActivityWorkspaceManager{Resolver: resolver, Kubernetes: kubernetesTransfer},
			CloudAllocator: controlexecution.QueuedCloudAllocator{Cloud: cloud, Operations: cloud, Queue: events, PollInterval: 500 * time.Millisecond},
		},
	)
	if err != nil {
		return err
	}
	if err := supervisor.RecoverWorkspaces(context.Background()); err != nil {
		return fmt.Errorf("recover activity workspaces: %w", err)
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

func sharedLocalCapacity() (ports.LocalActivityCapacity, error) {
	cpuValue, memoryValue := os.Getenv("AKOFLOW_SHARED_LOCAL_CPUS"), os.Getenv("AKOFLOW_SHARED_LOCAL_MEMORY_BYTES")
	if cpuValue == "" && memoryValue == "" {
		return ports.LocalActivityCapacity{}, nil
	}
	cpus, cpuErr := strconv.ParseFloat(cpuValue, 64)
	memory, memoryErr := strconv.ParseInt(memoryValue, 10, 64)
	if cpuErr != nil || memoryErr != nil || cpus <= 0 || math.IsNaN(cpus) || math.IsInf(cpus, 0) || memory <= 0 {
		return ports.LocalActivityCapacity{}, fmt.Errorf("set AKOFLOW_SHARED_LOCAL_CPUS and AKOFLOW_SHARED_LOCAL_MEMORY_BYTES to positive capacities")
	}
	return ports.LocalActivityCapacity{CPU: cpus, MemoryBytes: memory}, nil
}

func reconcileLocalReservations(ctx context.Context, store ports.LocalAdmissionStore, executor provider.CommandExecutor) error {
	tasks, err := store.ListLocalReconciliations(ctx)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		released := false
		if task.Docker {
			name := localprovider.ExecutionContainerName(task.RunID, task.ActivityID, task.Attempt)
			released = localprovider.ContainerReservationReleased(ctx, executor, name)
		} else {
			released = (task.NativeGated && task.ProcessID == "" && task.ProcessIdentity == "") ||
				localprovider.NativeReservationReleased(task.ProcessID, task.ProcessIdentity)
		}
		if released {
			if err := store.ReleaseReconciledLocalActivity(ctx, task.TaskID); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}
