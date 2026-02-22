package payment

import (
	"fmt"
	"time"

	"github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/google/uuid"
)

// PaymentType represents the type of payment
type PaymentType string

const (
	InternalTransfer PaymentType = "internal_transfer"
	ExternalPayment  PaymentType = "external_payment"
)

// PaymentStatus represents the payment status in the state machine
type PaymentStatus string

const (
	StatusPending    PaymentStatus = "pending"
	StatusProcessing PaymentStatus = "processing"
	StatusCompleted  PaymentStatus = "completed"
	StatusFailed     PaymentStatus = "failed"
	StatusCancelled  PaymentStatus = "cancelled"
	StatusRefunded   PaymentStatus = "refunded"
)

// Provider represents the external payment provider
type Provider string

const (
	ProviderStripe Provider = "stripe"
	ProviderPayPal Provider = "paypal"
)

// Payment represents a payment entity
type Payment struct {
	ID                     uuid.UUID
	IdempotencyKey         string
	PaymentType            PaymentType
	SourceAccountID        *uuid.UUID
	DestinationAccountID   *uuid.UUID
	Amount                 Amount
	Status                 PaymentStatus
	Provider               *Provider
	ProviderTransactionID  *string
	RetryCount             int
	MaxRetries             int
	LastError              *string
	SagaID                 *uuid.UUID
	SagaStep               int
	Metadata               map[string]interface{}
	CreatedAt              time.Time
	UpdatedAt              time.Time
	CompletedAt            *time.Time
}

// Amount represents a monetary amount in the smallest currency unit (e.g. cents).
type Amount struct {
	ValueCents int64
	Currency   string
}

// String returns a human-readable representation of the amount.
func (a Amount) String() string {
	whole := a.ValueCents / 100
	frac := a.ValueCents % 100
	if frac < 0 {
		frac = -frac
	}
	return fmt.Sprintf("%d.%02d %s", whole, frac, a.Currency)
}

// Validate checks that the amount is valid.
func (a Amount) Validate() error {
	return validateAmount(a)
}

// NewPayment creates a new payment
func NewPayment(
	idempotencyKey string,
	paymentType PaymentType,
	sourceAccountID *uuid.UUID,
	destinationAccountID *uuid.UUID,
	amount Amount,
) (*Payment, error) {
	// Validate
	if err := validateAmount(amount); err != nil {
		return nil, err
	}

	if idempotencyKey == "" {
		return nil, errors.ErrInvalidInput
	}

	now := time.Now()
	return &Payment{
		ID:                   uuid.New(),
		IdempotencyKey:       idempotencyKey,
		PaymentType:          paymentType,
		SourceAccountID:      sourceAccountID,
		DestinationAccountID: destinationAccountID,
		Amount:               amount,
		Status:               StatusPending,
		RetryCount:           0,
		MaxRetries:           3,
		Metadata:             make(map[string]interface{}),
		CreatedAt:            now,
		UpdatedAt:            now,
	}, nil
}

// CanTransitionTo checks if the payment can transition to the given status
func (p *Payment) CanTransitionTo(newStatus PaymentStatus) bool {
	transitions := map[PaymentStatus][]PaymentStatus{
		StatusPending: {
			StatusProcessing,
			StatusCompleted, // For internal transfers (sync)
			StatusCancelled,
		},
		StatusProcessing: {
			StatusCompleted,
			StatusFailed,
		},
		StatusCompleted: {
			StatusRefunded,
		},
		StatusFailed: {
			StatusProcessing, // Retry
		},
		StatusCancelled: {},  // Terminal state
		StatusRefunded:  {},  // Terminal state
	}

	allowedTransitions, exists := transitions[p.Status]
	if !exists {
		return false
	}

	for _, allowed := range allowedTransitions {
		if allowed == newStatus {
			return true
		}
	}
	return false
}

// TransitionTo transitions the payment to a new status
func (p *Payment) TransitionTo(newStatus PaymentStatus) error {
	if !p.CanTransitionTo(newStatus) {
		return errors.NewDomainError(
			"invalid_transition",
			"cannot transition from "+string(p.Status)+" to "+string(newStatus),
			errors.ErrInvalidStateTransition,
		)
	}

	p.Status = newStatus
	p.UpdatedAt = time.Now()

	if newStatus == StatusCompleted || newStatus == StatusFailed || newStatus == StatusCancelled {
		now := time.Now()
		p.CompletedAt = &now
	}

	return nil
}

// MarkProcessing transitions the payment to processing status
func (p *Payment) MarkProcessing() error {
	return p.TransitionTo(StatusProcessing)
}

// MarkCompleted transitions the payment to completed status
func (p *Payment) MarkCompleted(providerTxID *string) error {
	if err := p.TransitionTo(StatusCompleted); err != nil {
		return err
	}
	if providerTxID != nil {
		p.ProviderTransactionID = providerTxID
	}
	return nil
}

// MarkFailed transitions the payment to failed status
func (p *Payment) MarkFailed(errorMsg string) error {
	if err := p.TransitionTo(StatusFailed); err != nil {
		return err
	}
	p.LastError = &errorMsg
	return nil
}

// MarkCancelled transitions the payment to cancelled status
func (p *Payment) MarkCancelled() error {
	return p.TransitionTo(StatusCancelled)
}

// MarkRefunded transitions the payment to refunded status
func (p *Payment) MarkRefunded() error {
	return p.TransitionTo(StatusRefunded)
}

// IncrementRetry increments the retry counter
func (p *Payment) IncrementRetry() error {
	if p.RetryCount >= p.MaxRetries {
		return errors.ErrMaxRetriesExceeded
	}
	p.RetryCount++
	p.UpdatedAt = time.Now()
	return nil
}

// CanRetry checks if the payment can be retried
func (p *Payment) CanRetry() bool {
	return p.Status == StatusFailed && p.RetryCount < p.MaxRetries
}

// IsTerminal checks if the payment is in a terminal state
func (p *Payment) IsTerminal() bool {
	return p.Status == StatusCompleted ||
		p.Status == StatusCancelled ||
		p.Status == StatusRefunded
}

// SetProvider sets the payment provider
func (p *Payment) SetProvider(provider Provider) {
	p.Provider = &provider
}

func validateAmount(amount Amount) error {
	if amount.ValueCents <= 0 {
		return errors.NewValidationError("amount", "must be greater than 0")
	}
	if amount.Currency == "" {
		return errors.NewValidationError("currency", "cannot be empty")
	}
	// Simple currency validation (3-letter code)
	if len(amount.Currency) != 3 {
		return errors.NewValidationError("currency", "must be a 3-letter ISO code")
	}
	return nil
}
