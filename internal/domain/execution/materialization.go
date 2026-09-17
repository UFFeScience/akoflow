package execution

import "fmt"

type MaterializationStatus string
type TransferStatus string
type CatalogScope string
type TransferStrategy string

const (
	CatalogScopeSystem          CatalogScope          = "system"
	CatalogScopeOrganization    CatalogScope          = "organization"
	CatalogScopeProject         CatalogScope          = "project"
	CatalogScopeWorkflow        CatalogScope          = "workflow"
	CatalogScopeRun             CatalogScope          = "run"
	CatalogScopeEnvironment     CatalogScope          = "environment"
	TransferUseExisting         TransferStrategy      = "use-existing"
	TransferDestinationPull     TransferStrategy      = "destination-pull"
	TransferSourcePush          TransferStrategy      = "source-push"
	TransferGateway             TransferStrategy      = "gateway"
	TransferRuntimeLocal        TransferStrategy      = "runtime-local"
	TransferSharedStorage       TransferStrategy      = "shared-storage"
	TransferDirectRuntime       TransferStrategy      = "direct-runtime"
	MaterializationPlanned      MaterializationStatus = "planned"
	MaterializationReconciling  MaterializationStatus = "reconciling"
	MaterializationTransferring MaterializationStatus = "transferring"
	MaterializationVerifying    MaterializationStatus = "verifying"
	MaterializationCommitted    MaterializationStatus = "committed"
	MaterializationFailed       MaterializationStatus = "failed"
	TransferPlanned             TransferStatus        = "planned"
	TransferRunning             TransferStatus        = "running"
	TransferCompleted           TransferStatus        = "completed"
	TransferFailed              TransferStatus        = "failed"
)

// ExecutableArtifact and ArtifactVariant describe immutable executable bytes.
// They intentionally differ from ArtifactManifest, which records activity output.
type ExecutableArtifact struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Version       string   `json:"version,omitempty"`
	Formats       []string `json:"formats,omitempty"`
	Architectures []string `json:"architectures,omitempty"`
	Available     bool     `json:"available"`
}
type ArtifactVariant struct {
	ID           string `json:"id"`
	ArtifactID   string `json:"artifactId"`
	Digest       string `json:"digest"`
	Format       string `json:"format"`
	Architecture string `json:"architecture,omitempty"`
	SizeBytes    int64  `json:"sizeBytes,omitempty"`
}
type ArtifactMaterialization struct {
	ID              string                `json:"id"`
	RunID           string                `json:"runId,omitempty"`
	ActivityID      string                `json:"activityId,omitempty"`
	VariantID       string                `json:"variantId"`
	Digest          string                `json:"digest"`
	ResourceID      string                `json:"resourceId"`
	EnvironmentID   string                `json:"environmentId,omitempty"`
	DestinationPath string                `json:"destinationPath"`
	Status          MaterializationStatus `json:"status"`
	VerifiedDigest  string                `json:"verifiedDigest,omitempty"`
}

func (m ArtifactMaterialization) Committed() bool {
	return m.Status == MaterializationCommitted && m.VerifiedDigest != "" && m.VerifiedDigest == m.Digest
}

type TransferLocation struct {
	ResourceID      string `json:"resourceId,omitempty"`
	EnvironmentID   string `json:"environmentId,omitempty"`
	RuntimeID       string `json:"runtimeId,omitempty"`
	ConnectionID    string `json:"connectionId,omitempty"`
	CloudInstanceID string `json:"cloudInstanceId,omitempty"`
	NetworkDomain   string `json:"networkDomain,omitempty"`
	URI             string `json:"uri"`
	Path            string `json:"path,omitempty"`
}
type ArtifactVersion struct {
	ID         string       `json:"id"`
	ArtifactID string       `json:"artifactId"`
	Version    string       `json:"version"`
	Scope      CatalogScope `json:"scope"`
	ScopeID    string       `json:"scopeId,omitempty"`
}
type ArtifactLocation struct {
	ID         string       `json:"id"`
	VariantID  string       `json:"variantId"`
	Scope      CatalogScope `json:"scope"`
	ScopeID    string       `json:"scopeId,omitempty"`
	EndpointID string       `json:"endpointId"`
	URI        string       `json:"uri"`
	Digest     string       `json:"digest"`
	Available  bool         `json:"available"`
}
type TransferEndpoint struct {
	ID              string            `json:"id"`
	Kind            string            `json:"kind"`
	URI             string            `json:"uri"`
	ResourceID      string            `json:"resourceId,omitempty"`
	EnvironmentID   string            `json:"environmentId,omitempty"`
	RuntimeID       string            `json:"runtimeId,omitempty"`
	ConnectionID    string            `json:"connectionId,omitempty"`
	CloudInstanceID string            `json:"cloudInstanceId,omitempty"`
	NetworkDomain   string            `json:"networkDomain,omitempty"`
	Configuration   map[string]string `json:"configuration,omitempty"`
}

type TransferRoute struct {
	Strategy              TransferStrategy `json:"strategy"`
	ProducerActivityID    string           `json:"producerActivityId,omitempty"`
	ConsumerActivityID    string           `json:"consumerActivityId,omitempty"`
	SourceResourceID      string           `json:"sourceResourceId,omitempty"`
	TargetResourceID      string           `json:"targetResourceId,omitempty"`
	SourceEnvironmentID   string           `json:"sourceEnvironmentId,omitempty"`
	TargetEnvironmentID   string           `json:"targetEnvironmentId,omitempty"`
	SourceAddress         string           `json:"sourceAddress,omitempty"`
	TargetAddress         string           `json:"targetAddress,omitempty"`
	SourceCloudInstanceID string           `json:"sourceCloudInstanceId,omitempty"`
	TargetCloudInstanceID string           `json:"targetCloudInstanceId,omitempty"`
	FilesTransferred      int64            `json:"filesTransferred,omitempty"`
	NetworkDomain         string           `json:"networkDomain,omitempty"`
	Fallback              TransferStrategy `json:"fallback,omitempty"`
	Reason                string           `json:"reason"`
}
type ConnectorBinding struct {
	ID            string `json:"id"`
	EndpointID    string `json:"endpointId"`
	Connector     string `json:"connector"`
	CredentialRef string `json:"credentialRef,omitempty"`
	Enabled       bool   `json:"enabled"`
}
type BlobDescriptor struct {
	Digest    string `json:"digest"`
	SizeBytes int64  `json:"sizeBytes"`
	Path      string `json:"path,omitempty"`
}
type TransferChunk struct {
	Index     int    `json:"index"`
	Offset    int64  `json:"offset"`
	SizeBytes int64  `json:"sizeBytes"`
	Digest    string `json:"digest,omitempty"`
	Received  bool   `json:"received,omitempty"`
}
type TransferChunkRun struct {
	TransferRunID string         `json:"transferRunId"`
	Index         int            `json:"index"`
	Offset        int64          `json:"offset"`
	SizeBytes     int64          `json:"sizeBytes"`
	Digest        string         `json:"digest,omitempty"`
	Status        TransferStatus `json:"status"`
	Attempts      int            `json:"attempts"`
}
type DataTransferPlan struct {
	ID                 string           `json:"id"`
	ExecutionRunID     string           `json:"executionRunId,omitempty"`
	ProducerActivityID string           `json:"producerActivityId,omitempty"`
	ConsumerActivityID string           `json:"consumerActivityId,omitempty"`
	Strategy           TransferStrategy `json:"strategy,omitempty"`
	Source             TransferLocation `json:"source"`
	Destination        TransferLocation `json:"destination"`
	Blobs              []BlobDescriptor `json:"blobs"`
	Chunks             []TransferChunk  `json:"chunks,omitempty"`
	ResumeFrom         []int            `json:"resumeFrom,omitempty"`
	Route              TransferRoute    `json:"route,omitempty"`
	SyncWorkspace      bool             `json:"syncWorkspace,omitempty"`
}
type DataTransferRun struct {
	ID               string           `json:"id"`
	PlanID           string           `json:"planId"`
	ExecutionRunID   string           `json:"executionRunId,omitempty"`
	ActivityID       string           `json:"activityId,omitempty"`
	Strategy         TransferStrategy `json:"strategy"`
	Status           TransferStatus   `json:"status"`
	VerifiedBlobs    []string         `json:"verifiedBlobs,omitempty"`
	CompletedChunks  []int            `json:"completedChunks,omitempty"`
	StartedAt        float64          `json:"startedAt,omitempty"`
	FinishedAt       float64          `json:"finishedAt,omitempty"`
	DurationSeconds  float64          `json:"durationSeconds,omitempty"`
	TransferredBytes int64            `json:"transferredBytes,omitempty"`
	Error            string           `json:"error,omitempty"`
	Route            TransferRoute    `json:"route,omitempty"`
	LogicalBytes     int64            `json:"logicalBytes,omitempty"`
	NetworkBytes     int64            `json:"networkBytes,omitempty"`
	FilesTransferred int64            `json:"filesTransferred,omitempty"`
}

// ArtifactBuild is an immutable build specification. Context and recipe are
// addressed by digest so retries are reproducible.
type ArtifactBuild struct {
	ID                 string `json:"id"`
	ArtifactVersionID  string `json:"artifactVersionId"`
	SourceType         string `json:"sourceType"`
	ContextDigest      string `json:"contextDigest"`
	RecipePath         string `json:"recipePath,omitempty"`
	RecipeDigest       string `json:"recipeDigest"`
	TargetFormat       string `json:"targetFormat"`
	TargetOS           string `json:"targetOs"`
	TargetArchitecture string `json:"targetArchitecture"`
	BuildArguments     string `json:"buildArguments,omitempty"`
	CacheKey           string `json:"cacheKey"`
}

type BuildRun struct {
	ID              string `json:"id"`
	ArtifactBuildID string `json:"artifactBuildId"`
	Status          string `json:"status"`
	OutputVariantID string `json:"outputVariantId,omitempty"`
	OutputDigest    string `json:"outputDigest,omitempty"`
	Logs            string `json:"logs,omitempty"`
	Error           string `json:"error,omitempty"`
}

// BuildContextArtifact identifies an uploaded, immutable build context. The
// bytes are owned by the artifact store; the database stores only its metadata.
type BuildContextArtifact struct {
	Digest     string `json:"digest"`
	StorageURI string `json:"storageUri"`
	SizeBytes  int64  `json:"sizeBytes"`
	MediaType  string `json:"mediaType,omitempty"`
}

type WorkspaceBlob struct {
	Path      string `json:"path"`
	Digest    string `json:"digest"`
	SizeBytes int64  `json:"sizeBytes"`
	Deleted   bool   `json:"deleted,omitempty"`
}
type WorkspaceRevision struct {
	ID       string          `json:"id"`
	ParentID string          `json:"parentId,omitempty"`
	Blobs    []WorkspaceBlob `json:"blobs"`
}
type WorkspaceInventory struct {
	Digests map[string]bool `json:"digests"`
}
type WorkspaceMaterialization struct {
	ID          string                `json:"id"`
	RevisionID  string                `json:"revisionId"`
	Destination TransferLocation      `json:"destination"`
	Status      MaterializationStatus `json:"status"`
	Missing     []BlobDescriptor      `json:"missing,omitempty"`
	Verified    []string              `json:"verified,omitempty"`
}

type PreparationRequirement struct {
	Artifact           *ArtifactMaterialization  `json:"artifact,omitempty"`
	ArtifactTransfer   *DataTransferPlan         `json:"artifactTransfer,omitempty"`
	Workspace          *WorkspaceMaterialization `json:"workspace,omitempty"`
	WorkspaceTransfer  *DataTransferPlan         `json:"workspaceTransfer,omitempty"`
	WorkspaceTransfers []DataTransferPlan        `json:"workspaceTransfers,omitempty"`
}

// ReconcileWorkspace returns the content-addressed blobs needed to construct
// desired at the destination. Deletions alter structure but carry no bytes.
func ReconcileWorkspace(desired WorkspaceRevision, inventory WorkspaceInventory) []BlobDescriptor {
	missing := make([]BlobDescriptor, 0)
	seen := make(map[string]bool)
	for _, blob := range desired.Blobs {
		if blob.Deleted || blob.Digest == "" || inventory.Digests[blob.Digest] || seen[blob.Digest] {
			continue
		}
		seen[blob.Digest] = true
		missing = append(missing, BlobDescriptor{Digest: blob.Digest, SizeBytes: blob.SizeBytes})
	}
	return missing
}
func (m *WorkspaceMaterialization) Reconcile(revision WorkspaceRevision, inventory WorkspaceInventory) {
	m.Status = MaterializationReconciling
	m.Missing = ReconcileWorkspace(revision, inventory)
}
func (m *WorkspaceMaterialization) Commit(verified []string) error {
	want, got := make(map[string]bool), make(map[string]bool)
	for _, b := range m.Missing {
		want[b.Digest] = true
	}
	for _, d := range verified {
		got[d] = true
	}
	for digest := range want {
		if !got[digest] {
			return fmt.Errorf("workspace materialization %s missing verified blob %s", m.ID, digest)
		}
	}
	m.Verified, m.Status = verified, MaterializationCommitted
	return nil
}

// PreparationGate is the executor-facing barrier. Providers must not submit
// work before both artifact and workspace have a verified committed state.
type PreparationGate struct {
	Executable *ArtifactMaterialization
	Workspace  *WorkspaceMaterialization
	// TransferRuns are verified transfer observations performed while preparing
	// this activity. They are surfaced in the execution timeline and rollups.
	TransferRuns []DataTransferRun
}

func (g PreparationGate) Ready() error {
	if g.Executable != nil && !g.Executable.Committed() {
		return fmt.Errorf("executable materialization is not committed")
	}
	if g.Workspace != nil && g.Workspace.Status != MaterializationCommitted {
		return fmt.Errorf("workspace materialization is not committed")
	}
	return nil
}
