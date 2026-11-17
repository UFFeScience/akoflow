package preflight

import "time"

type Check struct {
	Code       string    `json:"code"`
	Stage      string    `json:"stage"`
	EntityID   string    `json:"entityId,omitempty"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	NextAction string    `json:"nextAction,omitempty"`
	Path       string    `json:"path,omitempty"`
	ObservedAt time.Time `json:"observedAt"`
}

type Report struct {
	ID          string    `json:"id"`
	ContextHash string    `json:"contextHash"`
	Mode        string    `json:"mode"`
	Checks      []Check   `json:"checks"`
	Ready       bool      `json:"ready"`
	Warnings    int       `json:"warnings"`
	CreatedAt   time.Time `json:"createdAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type Input struct {
	StorageID          string `json:"storageId"`
	Path               string `json:"path"`
	ProducerActivityID string `json:"producerActivityId,omitempty"`
}
