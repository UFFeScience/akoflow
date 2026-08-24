package resource

import "time"

type StorageType string

const (
	StorageLocal  StorageType = "local"
	StoragePVC    StorageType = "pvc"
	StorageNFS    StorageType = "nfs"
	StorageS3     StorageType = "s3"
	StorageLustre StorageType = "lustre"
	StorageGCS    StorageType = "gcs"
	StorageMinIO  StorageType = "s3-compatible"
	StorageSSH    StorageType = "ssh-filesystem"
)

type StorageHealth string

const (
	StorageHealthUnknown     StorageHealth = "unknown"
	StorageHealthHealthy     StorageHealth = "healthy"
	StorageHealthDegraded    StorageHealth = "degraded"
	StorageHealthUnavailable StorageHealth = "unavailable"
)

type FileEntryType string

const (
	FileEntryFile      FileEntryType = "file"
	FileEntryDirectory FileEntryType = "directory"
	FileEntrySymlink   FileEntryType = "symlink"
)

type StorageCapabilities struct {
	Browse             bool `json:"browse"`
	Read               bool `json:"read"`
	Write              bool `json:"write"`
	Download           bool `json:"download"`
	Upload             bool `json:"upload"`
	Checksum           bool `json:"checksum"`
	PresignedDownload  bool `json:"presignedDownload"`
	ComputeNodeVisible bool `json:"computeNodeVisible"`
}
type StorageBrowseRoot struct {
	Path     string `json:"path"`
	Label    string `json:"label,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty"`
}
type StorageHealthStatus struct {
	Status         StorageHealth `json:"status"`
	Message        string        `json:"message,omitempty"`
	CheckedAt      time.Time     `json:"checkedAt,omitempty"`
	AvailableBytes int64         `json:"availableBytes,omitempty"`
}
type FileEntry struct {
	StorageID  string        `json:"storageId"`
	Path       string        `json:"path"`
	Name       string        `json:"name"`
	Type       FileEntryType `json:"type"`
	SizeBytes  int64         `json:"sizeBytes,omitempty"`
	ModifiedAt time.Time     `json:"modifiedAt,omitempty"`
	Checksum   string        `json:"checksum,omitempty"`
	ETag       string        `json:"etag,omitempty"`
	MediaType  string        `json:"mediaType,omitempty"`
	Readable   bool          `json:"readable"`
	Writable   bool          `json:"writable"`
	LinkTarget string        `json:"linkTarget,omitempty"`
}
type BrowseRequest struct {
	Path   string `json:"path,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}
type BrowsePage struct {
	StorageID  string      `json:"storageId"`
	Path       string      `json:"path"`
	Entries    []FileEntry `json:"entries"`
	NextCursor string      `json:"nextCursor,omitempty"`
}
type IndexPolicy struct {
	Enabled        bool     `json:"enabled"`
	Roots          []string `json:"roots,omitempty"`
	MaxDepth       int      `json:"maxDepth,omitempty"`
	MaxEntries     int      `json:"maxEntries,omitempty"`
	Include        []string `json:"include,omitempty"`
	Exclude        []string `json:"exclude,omitempty"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty"`
}
type IndexStatus struct {
	Status         string    `json:"status"`
	IndexedEntries int       `json:"indexedEntries,omitempty"`
	LastIndexedAt  time.Time `json:"lastIndexedAt,omitempty"`
	Message        string    `json:"message,omitempty"`
}
type IndexRun struct {
	ID             string    `json:"id"`
	StorageID      string    `json:"storageId"`
	Status         string    `json:"status"`
	IndexedEntries int       `json:"indexedEntries"`
	Error          string    `json:"error,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	FinishedAt     time.Time `json:"finishedAt,omitempty"`
}
type DownloadStatus string

const (
	DownloadQueued    DownloadStatus = "queued"
	DownloadReady     DownloadStatus = "ready"
	DownloadStreaming DownloadStatus = "streaming"
	DownloadCompleted DownloadStatus = "completed"
	DownloadFailed    DownloadStatus = "failed"
)

type DownloadRun struct {
	ID               string         `json:"id"`
	StorageID        string         `json:"storageId"`
	Path             string         `json:"path"`
	Status           DownloadStatus `json:"status"`
	Strategy         string         `json:"strategy"`
	URL              string         `json:"url,omitempty"`
	SizeBytes        int64          `json:"sizeBytes,omitempty"`
	TransferredBytes int64          `json:"transferredBytes,omitempty"`
	Error            string         `json:"error,omitempty"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}

type StorageResource struct {
	ID                   string                  `json:"id"`
	EnvironmentVersionID string                  `json:"environmentVersionId"`
	Name                 string                  `json:"name"`
	Type                 StorageType             `json:"type"`
	Endpoint             string                  `json:"endpoint,omitempty"`
	CapacityBytes        int64                   `json:"capacityBytes"`
	Shared               bool                    `json:"shared"`
	ReadOnly             bool                    `json:"readOnly"`
	Configuration        map[string]any          `json:"configuration,omitempty"`
	CredentialReference  string                  `json:"credentialReference,omitempty"`
	Metadata             map[string]any          `json:"metadata,omitempty"`
	RuntimeBindings      []StorageRuntimeBinding `json:"runtimeBindings,omitempty"`
	BrowseRoots          []StorageBrowseRoot     `json:"browseRoots,omitempty"`
	Capabilities         StorageCapabilities     `json:"capabilities,omitempty"`
	Health               StorageHealthStatus     `json:"health,omitempty"`
	IndexPolicy          IndexPolicy             `json:"indexPolicy,omitempty"`
	IndexStatus          IndexStatus             `json:"indexStatus,omitempty"`
}

// StorageRuntimeBinding declares that a runtime can access a storage resource.
// Default is used when an activity does not explicitly select a storage.
type StorageRuntimeBinding struct {
	RuntimeID     string         `json:"runtimeId"`
	Default       bool           `json:"default,omitempty"`
	HostPath      string         `json:"hostPath,omitempty"`
	ContainerPath string         `json:"containerPath,omitempty"`
	ReadOnly      bool           `json:"readOnly,omitempty"`
	Configuration map[string]any `json:"configuration,omitempty"`
}
