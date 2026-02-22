package payment

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TransactionManager defines the interface for transaction management.
// This is an application-layer port, not a domain concern.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// OutboxWriter defines the interface for writing to the transactional outbox.
type OutboxWriter interface {
	Insert(ctx context.Context, entry *OutboxEntry) error
}

// OutboxEntry represents a transactional outbox record.
type OutboxEntry struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Payload       map[string]interface{}
	Status        string
	RetryCount    int
	MaxRetries    int
	CreatedAt     time.Time
	PublishedAt   *time.Time
}
