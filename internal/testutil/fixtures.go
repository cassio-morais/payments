package testutil

import (
	"time"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/google/uuid"
)

// NewTestAccount creates a test account with sensible defaults.
func NewTestAccount(userID string, balanceCents int64, currency string) *account.Account {
	now := time.Now()
	return &account.Account{
		ID:        uuid.New(),
		UserID:    userID,
		Balance:   balanceCents,
		Currency:  currency,
		Version:   0,
		Status:    account.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewTestPayment creates a pending test payment with sensible defaults.
func NewTestPayment(
	paymentType payment.PaymentType,
	sourceID *uuid.UUID,
	destID *uuid.UUID,
	amountCents int64,
	currency string,
) *payment.Payment {
	now := time.Now()
	return &payment.Payment{
		ID:                   uuid.New(),
		IdempotencyKey:       uuid.New().String(),
		PaymentType:          paymentType,
		SourceAccountID:      sourceID,
		DestinationAccountID: destID,
		Amount:               payment.Amount{ValueCents: amountCents, Currency: currency},
		Status:               payment.StatusPending,
		RetryCount:           0,
		MaxRetries:           3,
		Metadata:             make(map[string]interface{}),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

// NewCompletedPayment creates a completed test payment.
func NewCompletedPayment(
	paymentType payment.PaymentType,
	sourceID *uuid.UUID,
	destID *uuid.UUID,
	amountCents int64,
	currency string,
) *payment.Payment {
	p := NewTestPayment(paymentType, sourceID, destID, amountCents, currency)
	p.Status = payment.StatusCompleted
	completedAt := time.Now()
	p.CompletedAt = &completedAt
	return p
}

// UUIDPtr returns a pointer to the UUID.
func UUIDPtr(id uuid.UUID) *uuid.UUID {
	return &id
}
