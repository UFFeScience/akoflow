package environment

import "time"

// TransferConnector identifies a protocol that can move artifacts to or from
// an environment.  It deliberately describes a capability, not a storage
// product: an S3-compatible endpoint can be MinIO, Ceph or AWS S3.
type TransferConnector string

const (
	TransferConnectorRsync TransferConnector = "rsync"
	TransferConnectorSCP   TransferConnector = "scp"
	TransferConnectorSFTP  TransferConnector = "sftp"
	TransferConnectorHTTP  TransferConnector = "http"
	TransferConnectorS3    TransferConnector = "s3-compatible"
	TransferConnectorGCS   TransferConnector = "gcs"
)

type CapabilityObservation struct {
	Available  bool      `json:"available"`
	Declared   *bool     `json:"declared,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	ObservedAt time.Time `json:"observedAt,omitempty"`
	FreshUntil time.Time `json:"freshUntil,omitempty"`
}

func (c CapabilityObservation) Fresh(at time.Time) bool {
	return c.ObservedAt.IsZero() || c.FreshUntil.IsZero() || !at.After(c.FreshUntil)
}

// TransferPath describes a path the login node can write.  ComputeNodeVisible
// is only true after an explicit scheduler probe; absence means unknown.
type TransferPath struct {
	Path               string    `json:"path"`
	Kind               string    `json:"kind"` // home, scratch, temporary, configured
	Writable           bool      `json:"writable"`
	AvailableBytes     int64     `json:"availableBytes"`
	LoginNodeVisible   bool      `json:"loginNodeVisible"`
	ComputeNodeVisible *bool     `json:"computeNodeVisible,omitempty"`
	Reason             string    `json:"reason,omitempty"`
	ObservedAt         time.Time `json:"observedAt,omitempty"`
}

type TransferCapabilities struct {
	Connectors       map[TransferConnector]CapabilityObservation `json:"connectors,omitempty"`
	OutboundHTTPS    CapabilityObservation                       `json:"outboundHttps"`
	ContainerRuntime CapabilityObservation                       `json:"containerRuntime"`
	Checksum         CapabilityObservation                       `json:"checksum"`
	Paths            []TransferPath                              `json:"paths,omitempty"`
	ObservedAt       time.Time                                   `json:"observedAt,omitempty"`
	FreshUntil       time.Time                                   `json:"freshUntil,omitempty"`
}

type ConnectorBinding struct {
	ID            string            `json:"id"`
	EnvironmentID string            `json:"environmentId"`
	Connector     TransferConnector `json:"connector"`
	Endpoint      string            `json:"endpoint,omitempty"`
	CredentialRef string            `json:"credentialRef,omitempty"`
	Configuration map[string]any    `json:"configuration,omitempty"`
	Health        ConnectorHealth   `json:"health"`
}

// ConnectorHealth is intentionally attached to a binding, not discovery:
// credentials must only be tested when the user explicitly requests it.
type ConnectorHealth struct {
	Healthy    bool      `json:"healthy"`
	Operation  string    `json:"operation,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	CheckedAt  time.Time `json:"checkedAt,omitempty"`
	FreshUntil time.Time `json:"freshUntil,omitempty"`
}

type ArtifactLocationHealth struct {
	LocationRef         string    `json:"locationRef"`
	Exists              bool      `json:"exists"`
	SizeBytes           int64     `json:"sizeBytes"`
	ExpectedSizeBytes   int64     `json:"expectedSizeBytes,omitempty"`
	Digest              string    `json:"digest,omitempty"`
	ExpectedDigest      string    `json:"expectedDigest,omitempty"`
	LoginNodeReadable   bool      `json:"loginNodeReadable"`
	ComputeNodeReadable *bool     `json:"computeNodeReadable,omitempty"`
	Reason              string    `json:"reason,omitempty"`
	CheckedAt           time.Time `json:"checkedAt,omitempty"`
}
