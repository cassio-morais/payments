package service

import (
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/google/uuid"
)


// CreateAccountRequest holds the service layer input for creating an account.
// This uses business domain types (int64 cents, UUIDs) rather than HTTP types.
// Controllers convert their HTTP DTOs to this type.
type CreateAccountRequest struct {
	UserID         string
	InitialBalance int64 // in cents
	Currency       string
}


// CreatePaymentRequest holds the service layer input for creating a payment.
// This uses business domain types (int64 cents, UUID, domain enums) rather than HTTP types.
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

// CreatePaymentResponse holds the result of creating a payment.
type CreatePaymentResponse struct {
	Payment *payment.Payment
	IsAsync bool
}

// TransferRequest is a simplified request for internal transfers.
type TransferRequest struct {
	IdempotencyKey       string
	SourceAccountID      uuid.UUID
	DestinationAccountID uuid.UUID
	Amount               int64 // in cents
	Currency             string
}
