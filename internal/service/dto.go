package service

import (
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/google/uuid"
)


// Controllers convert their HTTP DTOs to this type.
type CreateAccountRequest struct {
	UserID         string
	InitialBalance int64 // in cents
	Currency       string
}


// Controllers convert their HTTP DTOs to this type.
type CreatePaymentRequest struct {
	IdempotencyKey       string
	PaymentType          payment.PaymentType
	SourceAccountID      *uuid.UUID
	DestinationAccountID *uuid.UUID
	Amount               int64 // in cents
	Currency             string
	Provider             *payment.Provider
}

type CreatePaymentResponse struct {
	Payment *payment.Payment
	IsAsync bool
}

type TransferRequest struct {
	IdempotencyKey       string
	SourceAccountID      uuid.UUID
	DestinationAccountID uuid.UUID
	Amount               int64 // in cents
	Currency             string
}
