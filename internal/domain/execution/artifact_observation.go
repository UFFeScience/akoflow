package execution

type ArtifactChange string

const (
	ArtifactCreated  ArtifactChange = "created"
	ArtifactModified ArtifactChange = "modified"
	ArtifactDeleted  ArtifactChange = "deleted"
)

// ArtifactObservation records runtime evidence. It does not change the
// workflow declaration; learned profiles are derived from many observations.
type ArtifactObservation struct {
	Path             string         `json:"path"`
	Change           ArtifactChange `json:"change"`
	SizeBytes        int64          `json:"sizeBytes"`
	Checksum         string         `json:"checksum,omitempty"`
	ModifiedUnixNano int64          `json:"modifiedUnixNano,omitempty"`
}

// LifecycleObservation describes a lightweight observer phase without
// sampling the activity or requiring utilities inside its image.
type LifecycleObservation struct {
	Phase           string  `json:"phase"`
	Status          string  `json:"status"`
	StartedAt       float64 `json:"startedAt"`
	FinishedAt      float64 `json:"finishedAt"`
	DurationSeconds float64 `json:"durationSeconds"`
	Error           string  `json:"error,omitempty"`
}

type ArtifactSummary struct {
	InitialFiles  int   `json:"initialFiles"`
	FinalFiles    int   `json:"finalFiles"`
	CreatedFiles  int   `json:"createdFiles"`
	ModifiedFiles int   `json:"modifiedFiles"`
	DeletedFiles  int   `json:"deletedFiles"`
	OutputBytes   int64 `json:"outputBytes"`
}

// ArtifactManifest is the portable JSON document produced by a runtime
// observer for one concrete activity attempt.
type ArtifactManifest struct {
	SchemaVersion int                    `json:"schemaVersion"`
	RunID         string                 `json:"runId"`
	ActivityID    string                 `json:"activityId"`
	Attempt       int                    `json:"attempt"`
	Runtime       string                 `json:"runtime"`
	Hostname      string                 `json:"hostname,omitempty"`
	Root          string                 `json:"root"`
	StartedAt     float64                `json:"startedAt"`
	FinishedAt    float64                `json:"finishedAt"`
	ExitCode      int                    `json:"exitCode"`
	Files         []ArtifactObservation  `json:"files"`
	Phases        []LifecycleObservation `json:"phases,omitempty"`
	Summary       ArtifactSummary        `json:"summary"`
}
