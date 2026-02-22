package outbox

import (
	"time"

	"github.com/google/uuid"
)

// Entry represents a transactional outbox record for reliable event publishing.
type Entry struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       map[string]interface{}
	Status        Status
	RetryCount    int
	MaxRetries    int
	CreatedAt     time.Time
	PublishedAt   *time.Time
}

// Status represents the outbox entry status.
type Status string

const (
	StatusPending   Status = "pending"
	StatusPublished Status = "published"
	StatusFailed    Status = "failed"
)

// NewEntry creates a new outbox entry in pending status.
func NewEntry(aggregateType string, aggregateID uuid.UUID, eventType string, payload map[string]interface{}) *Entry {
	return &Entry{
		ID:            uuid.New(),
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payload,
		Status:        StatusPending,
		RetryCount:    0,
		MaxRetries:    5,
		CreatedAt:     time.Now(),
	}
}
