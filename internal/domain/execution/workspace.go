package execution

import "time"

type WorkspaceState string

const (
	WorkspacePlanned    WorkspaceState = "planned"
	WorkspaceActive     WorkspaceState = "active"
	WorkspaceSealed     WorkspaceState = "sealed"
	WorkspaceReleasable WorkspaceState = "releasable"
	WorkspaceReleasing  WorkspaceState = "releasing"
	WorkspaceReleased   WorkspaceState = "released"
	WorkspaceFailed     WorkspaceState = "failed"
)

type WorkspaceRetention string

const (
	WorkspaceIntermediate WorkspaceRetention = "intermediate"
	WorkspaceFinal        WorkspaceRetention = "final"
	WorkspacePinned       WorkspaceRetention = "pinned"
)

type WorkspaceEntry struct {
	Path      string `json:"path"`
	Digest    string `json:"digest,omitempty"`
	SizeBytes int64  `json:"sizeBytes"`
}

type WorkspaceManifest struct {
	Initial   []WorkspaceEntry `json:"initial,omitempty"`
	Final     []WorkspaceEntry `json:"final,omitempty"`
	Inputs    []WorkspaceEntry `json:"inputs,omitempty"`
	Outputs   []WorkspaceEntry `json:"outputs,omitempty"`
	Removed   []WorkspaceEntry `json:"removed,omitempty"`
	Temporary []WorkspaceEntry `json:"temporary,omitempty"`
}

type ActivityWorkspace struct {
	ID              string             `json:"id"`
	RunID           string             `json:"runId"`
	ActivityID      string             `json:"activityId"`
	EnvironmentID   string             `json:"environmentId,omitempty"`
	ResourceID      string             `json:"resourceId"`
	RuntimeID       string             `json:"runtimeId"`
	ConnectionID    string             `json:"connectionId,omitempty"`
	URI             string             `json:"uri"`
	ExecutionPath   string             `json:"executionPath"`
	ObservationPath string             `json:"observationPath"`
	Driver          string             `json:"driver"`
	State           WorkspaceState     `json:"state"`
	Retention       WorkspaceRetention `json:"retention"`
	IsFinal         bool               `json:"isFinal"`
	Pinned          bool               `json:"pinned"`
	Manifest        WorkspaceManifest  `json:"manifest"`
	FileCount       int64              `json:"fileCount"`
	SizeBytes       int64              `json:"sizeBytes"`
	InputBytes      int64              `json:"inputBytes"`
	OutputBytes     int64              `json:"outputBytes"`
	ReclaimedBytes  int64              `json:"reclaimedBytes"`
	ReleaseReason   string             `json:"releaseReason,omitempty"`
	LastError       string             `json:"lastError,omitempty"`
	CreatedAt       time.Time          `json:"createdAt"`
	SealedAt        *time.Time         `json:"sealedAt,omitempty"`
	ReleasedAt      *time.Time         `json:"releasedAt,omitempty"`
}

type WorkspaceLease struct {
	ID                 string     `json:"id"`
	WorkspaceID        string     `json:"workspaceId"`
	RunID              string     `json:"runId"`
	ProducerActivityID string     `json:"producerActivityId"`
	ConsumerActivityID string     `json:"consumerActivityId"`
	Released           bool       `json:"released"`
	ReleasedAt         *time.Time `json:"releasedAt,omitempty"`
	ReleaseReason      string     `json:"releaseReason,omitempty"`
}

type WorkspaceUsage struct {
	FileCount int64 `json:"fileCount"`
	SizeBytes int64 `json:"sizeBytes"`
}

type WorkspaceReleaseResult struct {
	ReclaimedBytes int64 `json:"reclaimedBytes"`
	RemovedFiles   int64 `json:"removedFiles"`
	PreservedFiles int64 `json:"preservedFiles"`
}
