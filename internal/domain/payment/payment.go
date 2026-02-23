package payment

import (
	"fmt"
	"time"

	"github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/google/uuid"
)

type PaymentType string

const (
	InternalTransfer PaymentType = "internal_transfer"
	ExternalPayment  PaymentType = "external_payment"
)

type PaymentStatus string

const (
	StatusPending    PaymentStatus = "pending"
	StatusProcessing PaymentStatus = "processing"
	StatusCompleted  PaymentStatus = "completed"
	StatusFailed     PaymentStatus = "failed"
	StatusCancelled  PaymentStatus = "cancelled"
	StatusRefunded   PaymentStatus = "refunded"
)

type Provider string

const (
	ProviderStripe Provider = "stripe"
	ProviderPayPal Provider = "paypal"
)

type EventType string

const (
	EventPaymentCreated   EventType = "payment.created"
	EventPaymentCompleted EventType = "payment.completed"
	EventPaymentFailed    EventType = "payment.failed"
	EventPaymentRefunded  EventType = "payment.refunded"
)

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
	Metadata               map[string]any
	CreatedAt              time.Time
	UpdatedAt              time.Time
	CompletedAt            *time.Time
}

type Amount struct {
	ValueCents int64
	Currency   string
}

func (a Amount) String() string {
	whole := a.ValueCents / 100
	frac := a.ValueCents % 100
	if frac < 0 {
		frac = -frac
	}
	return fmt.Sprintf("%d.%02d %s", whole, frac, a.Currency)
}

func (a Amount) Validate() error {
	return validateAmount(a)
}

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
		Metadata:             make(map[string]any),
		CreatedAt:            now,
		UpdatedAt:            now,
	}, nil
}

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

func (p *Payment) MarkProcessing() error {
	return p.TransitionTo(StatusProcessing)
}

func (p *Payment) MarkCompleted(providerTxID *string) error {
	if err := p.TransitionTo(StatusCompleted); err != nil {
		return err
	}
	if providerTxID != nil {
		p.ProviderTransactionID = providerTxID
	}
	return nil
}

func (p *Payment) MarkFailed(errorMsg string) error {
	if err := p.TransitionTo(StatusFailed); err != nil {
		return err
	}
	p.LastError = &errorMsg
	return nil
}

func (p *Payment) MarkCancelled() error {
	return p.TransitionTo(StatusCancelled)
}

func (p *Payment) MarkRefunded() error {
	return p.TransitionTo(StatusRefunded)
}

func (p *Payment) IncrementRetry() error {
	if p.RetryCount >= p.MaxRetries {
		return errors.ErrMaxRetriesExceeded
	}
	p.RetryCount++
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Payment) CanRetry() bool {
	return p.Status == StatusFailed && p.RetryCount < p.MaxRetries
}

func (p *Payment) IsTerminal() bool {
	return p.Status == StatusCompleted ||
		p.Status == StatusCancelled ||
		p.Status == StatusRefunded
}

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
