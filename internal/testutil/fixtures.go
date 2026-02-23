package testutil

import (
	"time"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/google/uuid"
)

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
		Metadata:             make(map[string]any),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

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

func UUIDPtr(id uuid.UUID) *uuid.UUID {
	return &id
}
