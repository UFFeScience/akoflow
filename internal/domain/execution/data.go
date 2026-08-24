package execution

type DataLocationStatus string

const (
	DataLocationEphemeral DataLocationStatus = "ephemeral"
	DataLocationStaging   DataLocationStatus = "staging"
	DataLocationAvailable DataLocationStatus = "available"
	DataLocationFailed    DataLocationStatus = "failed"
	DataLocationDeleted   DataLocationStatus = "deleted"
)

type DataObject struct {
	ID                 string         `json:"id"`
	WorkflowVersionID  string         `json:"workflowVersionId"`
	ProducerActivityID string         `json:"producerActivityId,omitempty"`
	LogicalName        string         `json:"logicalName"`
	RelativePath       string         `json:"relativePath"`
	Declared           bool           `json:"declared"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

type DataObjectInstance struct {
	ID                 string         `json:"id"`
	DataObjectID       string         `json:"dataObjectId"`
	ExecutionRunID     string         `json:"executionRunId"`
	ProducerActivityID string         `json:"producerActivityId"`
	Attempt            int            `json:"attempt"`
	RelativePath       string         `json:"relativePath"`
	SizeBytes          int64          `json:"sizeBytes"`
	Checksum           string         `json:"checksum,omitempty"`
	MediaType          string         `json:"mediaType,omitempty"`
	Discovered         bool           `json:"discovered"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

type DataLocation struct {
	ID                   string             `json:"id"`
	DataObjectInstanceID string             `json:"dataObjectInstanceId"`
	StorageResourceID    string             `json:"storageResourceId,omitempty"`
	ResourceID           string             `json:"resourceId,omitempty"`
	ExecutionRunID       string             `json:"executionRunId"`
	URI                  string             `json:"uri"`
	Status               DataLocationStatus `json:"status"`
	Metadata             map[string]any     `json:"metadata,omitempty"`
}
