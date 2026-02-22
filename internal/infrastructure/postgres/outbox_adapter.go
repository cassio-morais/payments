package postgres

import (
	"context"

	paymentApp "github.com/cassiomorais/payments/internal/application/payment"
)

// OutboxAdapter adapts OutboxRepository to the application-layer OutboxWriter interface.
type OutboxAdapter struct {
	repo *OutboxRepository
}

// NewOutboxAdapter creates a new OutboxAdapter.
func NewOutboxAdapter(repo *OutboxRepository) *OutboxAdapter {
	return &OutboxAdapter{repo: repo}
}

// Insert converts an application-layer OutboxEntry to the infrastructure type and inserts it.
func (a *OutboxAdapter) Insert(ctx context.Context, entry *paymentApp.OutboxEntry) error {
	return a.repo.Insert(ctx, &OutboxEntry{
		ID:            entry.ID,
		AggregateType: entry.AggregateType,
		AggregateID:   entry.AggregateID,
		EventType:     entry.EventType,
		Payload:       entry.Payload,
		Status:        entry.Status,
		RetryCount:    entry.RetryCount,
		MaxRetries:    entry.MaxRetries,
		CreatedAt:     entry.CreatedAt,
		PublishedAt:   entry.PublishedAt,
	})
}
