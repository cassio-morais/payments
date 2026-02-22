package payment

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines the interface for payment persistence
type Repository interface {
	// Create creates a new payment
	Create(ctx context.Context, payment *Payment) error

	// GetByID retrieves a payment by ID
	GetByID(ctx context.Context, id uuid.UUID) (*Payment, error)

	// GetByIdempotencyKey retrieves a payment by idempotency key
	GetByIdempotencyKey(ctx context.Context, key string) (*Payment, error)

	// Update updates an existing payment
	Update(ctx context.Context, payment *Payment) error

	// List lists payments with filters
	List(ctx context.Context, filter ListFilter) ([]*Payment, error)

	// AddEvent adds a payment event for audit trail
	AddEvent(ctx context.Context, event *PaymentEvent) error

	// GetEvents retrieves events for a payment
	GetEvents(ctx context.Context, paymentID uuid.UUID) ([]*PaymentEvent, error)
}

// ListFilter defines filters for listing payments
type ListFilter struct {
	AccountID *uuid.UUID
	Status    *PaymentStatus
	Provider  *Provider
	Limit     int
	Offset    int
	SortBy    string
	SortOrder string
}

// PaymentEvent represents an event in the payment lifecycle
type PaymentEvent struct {
	ID        uuid.UUID
	PaymentID uuid.UUID
	EventType string
	EventData map[string]any
	CreatedAt time.Time
}
