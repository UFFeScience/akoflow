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

type BackupSettings struct {
	Destination   string `json:"destination"`
	IntervalHours int    `json:"intervalHours"`
	Retain        int    `json:"retain"`
}

type BackupResult struct {
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	Scheduled  bool      `json:"scheduled"`
	File       string    `json:"file,omitempty"`
	Removed    []string  `json:"removed,omitempty"`
	Error      string    `json:"error,omitempty"`
}

type BackupStatus struct {
	Files         []string       `json:"files,omitempty"`
	Settings      BackupSettings `json:"settings"`
	Last          *BackupResult  `json:"last,omitempty"`
	LastScheduled *BackupResult  `json:"lastScheduled,omitempty"`
	NextScheduled time.Time      `json:"nextScheduled,omitempty"`
}

type RestorePreview struct {
	Checksum             string           `json:"checksum"`
	ID                   string           `json:"id"`
	ExportedAt           time.Time        `json:"exportedAt"`
	ArchiveVersion       int              `json:"archiveVersion"`
	Current              map[string]int64 `json:"current"`
	Replacement          map[string]int64 `json:"replacement"`
	Excluded             []string         `json:"excluded"`
	ExternalDependencies map[string]int64 `json:"externalDependencies"`
}

type RestoreResult struct {
	CompletedAt  time.Time `json:"completedAt"`
	RecoveryFile string    `json:"recoveryFile,omitempty"`
	Error        string    `json:"error,omitempty"`
}
