package hubpublish

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const packageLimit = 8 << 20

type Options struct {
	Directory  string
	HubURL     string
	BrowserURL string
	Source     http.Handler
	ReadOnly   bool
	Client     *http.Client
}
type Manager struct {
	options  Options
	mu       sync.Mutex
	state    savedState
	previews map[string]preparedPublication
}
type savedState struct {
	ProfileShareSecret   string                    `json:"profileShareSecret,omitempty"`
	WorkflowSources      map[string]WorkflowSource `json:"workflowSources,omitempty"`
	Enabled              bool                      `json:"enabled"`
	HubURL               string                    `json:"hubUrl"`
	AccessToken          string                    `json:"accessToken,omitempty"`
	DeviceCode           string                    `json:"deviceCode,omitempty"`
	UserCode             string                    `json:"userCode,omitempty"`
	VerificationURL      string                    `json:"verificationUrl,omitempty"`
	DeviceExpiresAt      time.Time                 `json:"deviceExpiresAt,omitempty"`
	NextPollAt           time.Time                 `json:"nextPollAt,omitempty"`
	PollInterval         int                       `json:"pollInterval,omitempty"`
	DefaultPublicationID string                    `json:"defaultPublicationId,omitempty"`
}
type Identity struct {
	ID              string    `json:"id"`
	Username        string    `json:"username"`
	Email           string    `json:"email"`
	ProjectID       string    `json:"projectId"`
	ProjectName     string    `json:"projectName"`
	DestinationName string    `json:"destinationName"`
	Owner           string    `json:"owner"`
	ExpiresAt       time.Time `json:"expiresAt"`
}
type WorkflowSource struct {
	URI           string `json:"uri,omitempty"`
	LocalHash     string `json:"localHash,omitempty"`
	HubOrigin     string `json:"hubOrigin,omitempty"`
	PublicationID string `json:"publicationId"`
	Version       int    `json:"version"`
	Hash          string `json:"hash"`
}

type Status struct {
	WorkflowSource       *WorkflowSource `json:"workflowSource,omitempty"`
	Configured           bool            `json:"configured"`
	BrowserURL           string          `json:"browserUrl"`
	Enabled              bool            `json:"enabled"`
	Connected            bool            `json:"connected"`
	HubURL               string          `json:"hubUrl"`
	ReadOnly             bool            `json:"readOnly"`
	Identity             *Identity       `json:"identity,omitempty"`
	UserCode             string          `json:"userCode,omitempty"`
	VerificationURL      string          `json:"verificationUrl,omitempty"`
	ExpiresAt            *time.Time      `json:"expiresAt,omitempty"`
	DefaultPublicationID string          `json:"defaultPublicationId,omitempty"`
	Message              string          `json:"message,omitempty"`
}
type Publication struct {
	Authorship     string `json:"authorship"`
	AnonymousOwner string `json:"anonymousOwner"`
	ID             string `json:"id"`
	Title          string `json:"title"`
	Slug           string `json:"slug"`
	Description    string `json:"description"`
	Readme         string `json:"readme"`
	Visibility     string `json:"visibility"`
	Owner          string `json:"owner"`
	Kind           string `json:"kind"`
}
type publicationVersion struct {
	Number int `json:"number"`
}
type publicationInfo struct {
	Publication Publication          `json:"publication"`
	Versions    []publicationVersion `json:"versions"`
}
type importInput struct {
	Contribution  bool   `json:"contribution,omitempty"`
	TargetVersion int    `json:"targetVersion,omitempty"`
	BundleFile    string `json:"bundleFile"`
	BaseVersion   int    `json:"baseVersion"`
	Message       string `json:"message"`
}
type preparedPublication struct {
	Profile      *profileExecutionInput
	RunID        string
	VersionInput *publicationVersionInput
	Input        importInput
	Publication  Publication
	TokenHash    string
	ExpiresAt    time.Time
}
type Preview struct {
	EnvironmentSnapshot *publishedEnvironment `json:"environmentSnapshot,omitempty"`
	TargetVersion       int                   `json:"targetVersion,omitempty"`
	ID                  string                `json:"id"`
	Publication         Publication           `json:"publication"`
	RunID               string                `json:"runId"`
	WorkflowName        string                `json:"workflowName"`
	YAML                string                `json:"yaml"`
	Logs                []map[string]any      `json:"logs"`
	Files               []any                 `json:"files"`
	ExportedAt          time.Time             `json:"exportedAt"`
	Review              json.RawMessage       `json:"review"`
}
type Receipt struct {
	ContributionStatus string `json:"contributionStatus,omitempty"`
	URL                string `json:"url"`
	ExperimentID       string `json:"experimentId"`
	Version            int    `json:"version"`
	ResultCount        int    `json:"resultCount"`
}
type apiError struct {
	Status     int
	Code       string
	Message    string
	ExistingID string
}

func (e *apiError) Error() string { return e.Message }
func failure(status int, code, message string) error {
	return &apiError{Status: status, Code: code, Message: message}
}
