package dto

import (
	"time"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/google/uuid"
)

// AccountResponse is the HTTP response for an account.
type AccountResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    string    `json:"user_id"`
	Balance   float64   `json:"balance"`
	Currency  string    `json:"currency"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FromAccount maps a domain Account to an AccountResponse.
func FromAccount(a *account.Account) *AccountResponse {
	return &AccountResponse{
		ID:        a.ID,
		UserID:    a.UserID,
		Balance:   centsToFloat(a.Balance),
		Currency:  a.Currency,
		Status:    string(a.Status),
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

// BalanceResponse is the HTTP response for an account balance.
type BalanceResponse struct {
	AccountID uuid.UUID `json:"account_id"`
	Balance   float64   `json:"balance"`
	Currency  string    `json:"currency"`
}

// PaymentResponse is the HTTP response for a payment.
type PaymentResponse struct {
	ID                    uuid.UUID  `json:"id"`
	IdempotencyKey        string     `json:"idempotency_key"`
	PaymentType           string     `json:"payment_type"`
	SourceAccountID       *uuid.UUID `json:"source_account_id,omitempty"`
	DestinationAccountID  *uuid.UUID `json:"destination_account_id,omitempty"`
	Amount                float64    `json:"amount"`
	Currency              string     `json:"currency"`
	Status                string     `json:"status"`
	Provider              *string    `json:"provider,omitempty"`
	ProviderTransactionID *string    `json:"provider_transaction_id,omitempty"`
	RetryCount            int        `json:"retry_count"`
	LastError             *string    `json:"last_error,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
}

// FromPayment maps a domain Payment to a PaymentResponse.
func FromPayment(p *payment.Payment) *PaymentResponse {
	resp := &PaymentResponse{
		ID:                    p.ID,
		IdempotencyKey:        p.IdempotencyKey,
		PaymentType:           string(p.PaymentType),
		SourceAccountID:       p.SourceAccountID,
		DestinationAccountID:  p.DestinationAccountID,
		Amount:                centsToFloat(p.Amount.ValueCents),
		Currency:              p.Amount.Currency,
		Status:                string(p.Status),
		ProviderTransactionID: p.ProviderTransactionID,
		RetryCount:            p.RetryCount,
		LastError:             p.LastError,
		CreatedAt:             p.CreatedAt,
		UpdatedAt:             p.UpdatedAt,
		CompletedAt:           p.CompletedAt,
	}
	if p.Provider != nil {
		s := string(*p.Provider)
		resp.Provider = &s
	}
	return resp
}

// TransactionResponse is the HTTP response for an account transaction.
type TransactionResponse struct {
	ID              uuid.UUID  `json:"id"`
	AccountID       uuid.UUID  `json:"account_id"`
	PaymentID       *uuid.UUID `json:"payment_id,omitempty"`
	TransactionType string     `json:"transaction_type"`
	Amount          float64    `json:"amount"`
	BalanceAfter    float64    `json:"balance_after"`
	Description     string     `json:"description"`
	CreatedAt       time.Time  `json:"created_at"`
}

// FromTransaction maps a domain Transaction to a TransactionResponse.
func FromTransaction(tx *account.Transaction) *TransactionResponse {
	return &TransactionResponse{
		ID:              tx.ID,
		AccountID:       tx.AccountID,
		PaymentID:       tx.PaymentID,
		TransactionType: string(tx.TransactionType),
		Amount:          centsToFloat(tx.Amount),
		BalanceAfter:    centsToFloat(tx.BalanceAfter),
		Description:     tx.Description,
		CreatedAt:       tx.CreatedAt,
	}
}

// ErrorResponse is a standard error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// centsToFloat converts int64 cents to float64 for JSON output.
func centsToFloat(cents int64) float64 {
	return float64(cents) / 100.0
}
