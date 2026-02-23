package outbox

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	// Insert creates a new outbox entry (typically inside a transaction)
	Insert(ctx context.Context, entry *Entry) error

	// GetPending returns pending outbox entries up to the given limit
	GetPending(ctx context.Context, limit int) ([]*Entry, error)

	// MarkPublished marks an outbox entry as published
	MarkPublished(ctx context.Context, id uuid.UUID) error

	// MarkFailed marks an outbox entry as failed and increments retry count
	MarkFailed(ctx context.Context, id uuid.UUID) error
}
