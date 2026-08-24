package instance

import "time"

// Instance identifies one AkôFlow control-plane installation.
type Instance struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	Organization string    `json:"organization,omitempty"`
	Location     string    `json:"location,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// UserPreferences stores presentation choices for one locally identified
// control-plane user. Authentication currently uses a shared API token, so the
// client supplies a stable browser profile ID instead of a server account ID.
type UserPreferences struct {
	ClientID          string    `json:"clientId"`
	Theme             string    `json:"theme"`
	AnimationsEnabled bool      `json:"animationsEnabled"`
	UpdatedAt         time.Time `json:"updatedAt"`
}
