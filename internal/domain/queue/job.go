package queue

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusLeased    Status = "leased"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

const (
	CategoryEnvironment   = "environment"
	CategoryOrchestration = "orchestration"
	CategoryExecution     = "execution"
	CategoryMonitoring    = "monitoring"
	CategoryTransfer      = "transfer"
	CategoryMaintenance   = "maintenance"
)

type Job struct {
	ID             string     `json:"id"`
	Category       string     `json:"category"`
	Type           string     `json:"eventType"`
	AggregateType  string     `json:"aggregateType,omitempty"`
	AggregateID    string     `json:"aggregateId,omitempty"`
	Payload        []byte     `json:"-"`
	Status         Status     `json:"status"`
	Priority       int        `json:"priority"`
	AvailableAt    time.Time  `json:"availableAt"`
	LeaseOwner     string     `json:"leaseOwner,omitempty"`
	LeaseExpiresAt *time.Time `json:"leaseExpiresAt,omitempty"`
	Attempts       int        `json:"attempts"`
	MaxAttempts    int        `json:"maxAttempts"`
	IdempotencyKey string     `json:"idempotencyKey,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
}

func New(category, eventType string, payload []byte, now time.Time) (Job, error) {
	if category == "" || eventType == "" {
		return Job{}, errors.New("queue job category and type are required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Job{
		ID:          newID(),
		Category:    category,
		Type:        eventType,
		Payload:     append([]byte{}, payload...),
		Status:      StatusPending,
		AvailableAt: now,
		MaxAttempts: 5,
		CreatedAt:   now,
	}, nil
}

func (j Job) Validate() error {
	if j.ID == "" || j.Category == "" || j.Type == "" {
		return errors.New("queue job id, category and type are required")
	}
	if j.MaxAttempts < 1 {
		return errors.New("queue job max attempts must be positive")
	}
	return nil
}

func newID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
}
