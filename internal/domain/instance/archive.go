package instance

import "time"

type ArchiveInstance struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Description    string           `json:"description,omitempty"`
	Source         string           `json:"source"`
	Status         string           `json:"status"`
	ReadOnly       bool             `json:"readOnly"`
	CredentialsSet bool             `json:"credentialsSet"`
	DatabasePath   string           `json:"-"`
	ImportedAt     *time.Time       `json:"importedAt,omitempty"`
	ExportedAt     *time.Time       `json:"exportedAt,omitempty"`
	LastOpenedAt   *time.Time       `json:"lastOpenedAt,omitempty"`
	SchemaVersion  string           `json:"schemaVersion,omitempty"`
	AkoflowVersion string           `json:"akoflowVersion,omitempty"`
	Contents       map[string]int64 `json:"contents,omitempty"`
}

type ArchiveManifest struct {
	Format        string            `json:"format"`
	FormatVersion int               `json:"formatVersion"`
	ExportedAt    time.Time         `json:"exportedAt"`
	Instance      ArchiveInstance   `json:"instance"`
	Security      ArchiveSecurity   `json:"security"`
	Contents      map[string]int64  `json:"contents,omitempty"`
	Checksums     map[string]string `json:"checksums"`
}

type ArchiveSecurity struct {
	CredentialsIncluded bool `json:"credentialsIncluded"`
	CredentialsRedacted bool `json:"credentialsRedacted"`
}
