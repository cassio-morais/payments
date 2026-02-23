package outbox

import (
	"time"

	"github.com/google/uuid"
)

type Entry struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       map[string]any
	Status        Status
	RetryCount    int
	MaxRetries    int
	CreatedAt     time.Time
	PublishedAt   *time.Time
}

type Status string

const (
	StatusPending   Status = "pending"
	StatusPublished Status = "published"
	StatusFailed    Status = "failed"
)

func NewEntry(aggregateType string, aggregateID uuid.UUID, eventType string, payload map[string]any) *Entry {
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
