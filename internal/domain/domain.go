// Package domain is the stable facade for AkoFlow's bounded domain contexts.
// New behavior belongs in the workflow, environment, resource, planning or
// execution subpackage; aliases here keep application ports concise.
package domain

import (
	"github.com/UFFeScience/akoflow/internal/domain/environment"
	"github.com/UFFeScience/akoflow/internal/domain/execution"
	"github.com/UFFeScience/akoflow/internal/domain/planning"
	"github.com/UFFeScience/akoflow/internal/domain/resource"
	"github.com/UFFeScience/akoflow/internal/domain/simulation"
	"github.com/UFFeScience/akoflow/internal/domain/workflow"
)

type SimulationEngine = simulation.Engine
type SimulationModel = simulation.Model
type SimulationScenario = simulation.Scenario
type SimulationRun = simulation.Run

type EnvironmentVersionStatus = environment.EnvironmentVersionStatus
type EnvironmentStatus = environment.EnvironmentStatus
type ConnectionType = environment.ConnectionType
type Environment = environment.Environment
type EnvironmentVersion = environment.EnvironmentVersion
type EnvironmentRuntime = environment.EnvironmentRuntime
type RuntimeDriver = environment.RuntimeDriver
type RuntimeMode = environment.RuntimeMode
type ExecutionScope = environment.ExecutionScope
type EnvironmentConnection = environment.EnvironmentConnection
type ConnectionCheck = environment.ConnectionCheck
type TransferConnector = environment.TransferConnector
type CapabilityObservation = environment.CapabilityObservation
type TransferPath = environment.TransferPath
type TransferCapabilities = environment.TransferCapabilities
type ConnectorBinding = environment.ConnectorBinding
type ConnectorHealth = environment.ConnectorHealth
type ArtifactLocationHealth = environment.ArtifactLocationHealth
type ConnectionStatus = environment.ConnectionStatus
type EnvironmentCapabilities = environment.Capabilities
type DiscoveryRun = environment.DiscoveryRun
type EnvironmentDefinition = environment.Definition

const (
	EnvironmentVersionDraft     = environment.EnvironmentVersionDraft
	EnvironmentVersionPublished = environment.EnvironmentVersionPublished
	EnvironmentVersionRetired   = environment.EnvironmentVersionRetired
	EnvironmentDefined          = environment.EnvironmentDefined
	EnvironmentConnecting       = environment.EnvironmentConnecting
	EnvironmentConnected        = environment.EnvironmentConnected
	EnvironmentDiscovering      = environment.EnvironmentDiscovering
	EnvironmentReady            = environment.EnvironmentReady
	EnvironmentDegraded         = environment.EnvironmentDegraded
	EnvironmentUnreachable      = environment.EnvironmentUnreachable
	ConnectionSSH               = environment.ConnectionSSH
	ConnectionKubernetes        = environment.ConnectionKubernetes
	ConnectionCloud             = environment.ConnectionCloud
	ConnectionLocal             = environment.ConnectionLocal
	ConnectionAgent             = environment.ConnectionAgent
	ConnectionOnline            = environment.ConnectionOnline
	ConnectionOffline           = environment.ConnectionOffline
	TransferConnectorRsync      = environment.TransferConnectorRsync
	TransferConnectorSCP        = environment.TransferConnectorSCP
	TransferConnectorSFTP       = environment.TransferConnectorSFTP
	TransferConnectorHTTP       = environment.TransferConnectorHTTP
	TransferConnectorS3         = environment.TransferConnectorS3
	TransferConnectorGCS        = environment.TransferConnectorGCS
	RuntimeDriverSlurm          = environment.RuntimeDriverSlurm
	RuntimeDriverKubernetes     = environment.RuntimeDriverKubernetes
	RuntimeDriverSSH            = environment.RuntimeDriverSSH
	RuntimeDriverLocal          = environment.RuntimeDriverLocal
	RuntimeDriverServerless     = environment.RuntimeDriverServerless
	RuntimeDriverSimGrid        = environment.RuntimeDriverSimGrid
	RuntimeModeExecution        = environment.RuntimeModeExecution
	RuntimeModeSimulation       = environment.RuntimeModeSimulation
)

type ResourceType = resource.ResourceType
type ResourceExecutionTarget = resource.ResourceExecutionTarget
type Resource = resource.Resource
type ResourceRuntimeBinding = resource.ResourceRuntimeBinding
type ResourceRelation = resource.ResourceRelation
type ResourceRelationType = resource.ResourceRelationType
type ResourceSnapshot = resource.ResourceSnapshot
type NetworkLink = resource.NetworkLink
type NetworkTopology = resource.NetworkTopology

const (
	ResourceCluster               = resource.ResourceCluster
	ResourceNodePool              = resource.ResourceNodePool
	ResourceKubernetesMachine     = resource.ResourceKubernetesMachine
	ResourceHPCPartition          = resource.ResourceHPCPartition
	ResourceHPCMachine            = resource.ResourceHPCMachine
	ResourceCloudVM               = resource.ResourceCloudVM
	ResourceFogDevice             = resource.ResourceFogDevice
	ResourceLocalMachine          = resource.ResourceLocalMachine
	ResourceServerlessPlatform    = resource.ResourceServerlessPlatform
	ResourceServerlessFunction    = resource.ResourceServerlessFunction
	ResourceBatchQueue            = resource.ResourceBatchQueue
	ResourceKubernetesNamespace   = resource.ResourceKubernetesNamespace
	ResourceSlurmReservation      = resource.ResourceSlurmReservation
	ExecutionTargetBatch          = resource.ExecutionTargetBatch
	ExecutionTargetDirect         = resource.ExecutionTargetDirect
	ResourceRelationContains      = resource.ResourceRelationContains
	ResourceRelationMemberOf      = resource.ResourceRelationMemberOf
	ResourceRelationAccessibleVia = resource.ResourceRelationAccessibleVia
)

type WorkflowVersion = workflow.WorkflowVersion
type WorkflowDefinition = workflow.Definition
type ActivityType = workflow.ActivityType
type Activity = workflow.Activity
type ActivityKind = workflow.ActivityKind
type ActivityCapability = workflow.ActivityCapability
type ActivityCommand = workflow.ActivityCommand
type ExecutableReference = workflow.ExecutableReference
type ExecutableSource = workflow.ExecutableSource
type ExecutableDelivery = workflow.ExecutableDelivery
type ExecutableSourceType = workflow.ExecutableSourceType
type DeliveryStrategy = workflow.DeliveryStrategy
type ExecutableFormat = workflow.ExecutableFormat
type ResolvedExecutable = workflow.ResolvedExecutable
type ActivityResources = workflow.ActivityResources
type ServiceSpec = workflow.ServiceSpec
type ActivitySimulation = workflow.ActivitySimulation
type ActivityPolicy = workflow.ActivityPolicy
type ActivityDependency = workflow.ActivityDependency
type ActivityDataDependency = workflow.ActivityDataDependency
type ActivityResourceProfile = workflow.ActivityResourceProfile

type PlanningSource = planning.PlanningSource
type ExecutionMode = planning.ExecutionMode
type PredictedMetrics = planning.PredictedMetrics
type SchedulePlan = planning.SchedulePlan
type PlanAssignment = planning.PlanAssignment
type PlanningRequest = planning.PlanningRequest

const (
	PlanningSourcePlugin          = planning.PlanningSourcePlugin
	PlanningSourceImported        = planning.PlanningSourceImported
	ExecutionModeReal             = planning.ExecutionModeReal
	ExecutionModeSimulation       = planning.ExecutionModeSimulation
	ExecutionModeInteractive      = planning.ExecutionModeInteractive
	ExecutionRunWorkflow          = execution.ExecutionRunWorkflow
	ExecutionRunInteractive       = execution.ExecutionRunInteractive
	ExecutionRunStandalone        = execution.ExecutionRunStandalone
	ActivityKindTask              = workflow.ActivityKindTask
	ActivityKindService           = workflow.ActivityKindService
	ActivityKindInteractive       = workflow.ActivityKindInteractive
	ActivityCapabilityReal        = workflow.ActivityCapabilityReal
	ActivityCapabilitySimulation  = workflow.ActivityCapabilitySimulation
	ActivityCapabilityInteractive = workflow.ActivityCapabilityInteractive
)

type ExecutionRunStatus = execution.ExecutionRunStatus
type TaskExecutionStatus = execution.TaskExecutionStatus
type ExecutionRun = execution.ExecutionRun
type ExecutionRunPage = execution.ExecutionRunPage
type ExecutionRunKind = execution.ExecutionRunKind
type TaskExecution = execution.TaskExecution
type ExecutionMetrics = execution.ExecutionMetrics
type ExecutionTrace = execution.ExecutionTrace
type DataTransfer = execution.DataTransfer
type ActivityExecutionContext = execution.ActivityExecutionContext
type ActivityHandle = execution.ActivityHandle
type ActivityHandleStatus = execution.ActivityHandleStatus
type ArtifactChange = execution.ArtifactChange
type ArtifactObservation = execution.ArtifactObservation
type ArtifactManifest = execution.ArtifactManifest

// ArtifactManifest remains the backwards-compatible name for observed output.
type OutputObservationManifest = execution.ArtifactManifest
type ExecutableArtifact = execution.ExecutableArtifact
type ArtifactVariant = execution.ArtifactVariant
type ArtifactMaterialization = execution.ArtifactMaterialization
type WorkspaceRevision = execution.WorkspaceRevision
type WorkspaceBlob = execution.WorkspaceBlob
type WorkspaceInventory = execution.WorkspaceInventory
type WorkspaceMaterialization = execution.WorkspaceMaterialization
type DataTransferPlan = execution.DataTransferPlan
type DataTransferRun = execution.DataTransferRun
type ArtifactBuild = execution.ArtifactBuild
type BuildRun = execution.BuildRun
type BuildContextArtifact = execution.BuildContextArtifact
type TransferLocation = execution.TransferLocation
type BlobDescriptor = execution.BlobDescriptor
type TransferEndpoint = execution.TransferEndpoint
type TransferConnectorBinding = execution.ConnectorBinding
type ArtifactVersion = execution.ArtifactVersion
type ArtifactLocation = execution.ArtifactLocation
type CatalogScope = execution.CatalogScope
type TransferStrategy = execution.TransferStrategy
type PreparationGate = execution.PreparationGate
type PreparationRequirement = execution.PreparationRequirement
type LifecycleObservation = execution.LifecycleObservation
type ArtifactSummary = execution.ArtifactSummary
type StorageType = resource.StorageType
type DataLocationStatus = execution.DataLocationStatus
type StorageResource = resource.StorageResource
type StorageRuntimeBinding = resource.StorageRuntimeBinding
type StorageCapabilities = resource.StorageCapabilities
type StorageBrowseRoot = resource.StorageBrowseRoot
type StorageHealthStatus = resource.StorageHealthStatus
type StorageHealth = resource.StorageHealth
type FileEntry = resource.FileEntry
type FileEntryType = resource.FileEntryType
type BrowseRequest = resource.BrowseRequest
type BrowsePage = resource.BrowsePage
type DownloadRun = resource.DownloadRun
type DownloadStatus = resource.DownloadStatus
type IndexPolicy = resource.IndexPolicy
type IndexStatus = resource.IndexStatus
type IndexRun = resource.IndexRun
type DataObject = execution.DataObject
type DataObjectInstance = execution.DataObjectInstance
type DataLocation = execution.DataLocation

const (
	ExecutionRunCreated         = execution.ExecutionRunCreated
	ExecutionRunRunning         = execution.ExecutionRunRunning
	ExecutionRunCompleted       = execution.ExecutionRunCompleted
	ExecutionRunFailed          = execution.ExecutionRunFailed
	TimingSubmittedAt           = execution.TimingSubmittedAt
	TimingContainerStartedAt    = execution.TimingContainerStartedAt
	TaskBlocked                 = execution.TaskBlocked
	TaskReady                   = execution.TaskReady
	TaskPreparing               = execution.TaskPreparing
	TaskRunning                 = execution.TaskRunning
	TaskCompleted               = execution.TaskCompleted
	TaskFailed                  = execution.TaskFailed
	TaskCancelled               = execution.TaskCancelled
	HandleStarting              = execution.HandleStarting
	HandleRunning               = execution.HandleRunning
	HandleCompleted             = execution.HandleCompleted
	HandleFailed                = execution.HandleFailed
	HandleStopped               = execution.HandleStopped
	ArtifactCreated             = execution.ArtifactCreated
	ArtifactModified            = execution.ArtifactModified
	ArtifactDeleted             = execution.ArtifactDeleted
	StorageLocal                = resource.StorageLocal
	StoragePVC                  = resource.StoragePVC
	StorageNFS                  = resource.StorageNFS
	StorageS3                   = resource.StorageS3
	StorageLustre               = resource.StorageLustre
	StorageGCS                  = resource.StorageGCS
	StorageMinIO                = resource.StorageMinIO
	StorageSSH                  = resource.StorageSSH
	FileEntryFile               = resource.FileEntryFile
	FileEntryDirectory          = resource.FileEntryDirectory
	FileEntrySymlink            = resource.FileEntrySymlink
	DownloadQueued              = resource.DownloadQueued
	DownloadReady               = resource.DownloadReady
	DownloadStreaming           = resource.DownloadStreaming
	DownloadCompleted           = resource.DownloadCompleted
	DownloadFailed              = resource.DownloadFailed
	DataLocationEphemeral       = execution.DataLocationEphemeral
	DataLocationStaging         = execution.DataLocationStaging
	DataLocationAvailable       = execution.DataLocationAvailable
	DataLocationFailed          = execution.DataLocationFailed
	DataLocationDeleted         = execution.DataLocationDeleted
	MaterializationPlanned      = execution.MaterializationPlanned
	MaterializationReconciling  = execution.MaterializationReconciling
	MaterializationTransferring = execution.MaterializationTransferring
	MaterializationVerifying    = execution.MaterializationVerifying
	MaterializationCommitted    = execution.MaterializationCommitted
	MaterializationFailed       = execution.MaterializationFailed
	TransferRunning             = execution.TransferRunning
	TransferPlanned             = execution.TransferPlanned
	TransferCompleted           = execution.TransferCompleted
	TransferFailed              = execution.TransferFailed
	TransferGateway             = execution.TransferGateway
	TransferUseExisting         = execution.TransferUseExisting
	TransferDestinationPull     = execution.TransferDestinationPull
	TransferSourcePush          = execution.TransferSourcePush
	DeliveryAuto                = workflow.DeliveryAuto
	DeliveryManaged             = workflow.DeliveryManaged
	DeliveryUseInPlace          = workflow.DeliveryUseInPlace
	DeliveryDestinationPull     = workflow.DeliveryDestinationPull
	DeliveryGatewayTransfer     = workflow.DeliveryGatewayTransfer
	DeliveryBuildAndTransfer    = workflow.DeliveryBuildAndTransfer
	DeliveryPreferInPlace       = workflow.DeliveryPreferInPlace
	ExecutableSourceOCI         = workflow.ExecutableSourceOCI
	ExecutableSourceRemoteFile  = workflow.ExecutableSourceRemoteFile
	ExecutableFormatSIF         = workflow.ExecutableFormatSIF
)
