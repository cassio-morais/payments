package controller

import (
	"time"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/google/uuid"
)

// --- Request DTOs ---
// These DTOs handle HTTP/JSON concerns (float64 for money, string for IDs, validation tags).
// Controllers convert these to service layer DTOs before calling business logic.

// CreateAccountRequest holds the input for creating an account.
type CreateAccountRequest struct {
	UserID         string  `json:"user_id" validate:"required"`
	InitialBalance float64 `json:"initial_balance" validate:"gte=0"`
	Currency       string  `json:"currency" validate:"required,len=3"`
}

// CreatePaymentRequest holds the input for creating a payment.
type CreatePaymentRequest struct {
	PaymentType          string  `json:"payment_type" validate:"required,oneof=internal_transfer external_payment"`
	SourceAccountID      *string `json:"source_account_id,omitempty"`
	DestinationAccountID *string `json:"destination_account_id,omitempty"`
	Amount               float64 `json:"amount" validate:"required,gt=0"`
	Currency             string  `json:"currency" validate:"required,len=3"`
	Provider             *string `json:"provider,omitempty"`
}

// TransferRequest holds the input for creating an internal transfer.
type TransferRequest struct {
	SourceAccountID      string  `json:"source_account_id" validate:"required,uuid"`
	DestinationAccountID string  `json:"destination_account_id" validate:"required,uuid"`
	Amount               float64 `json:"amount" validate:"required,gt=0"`
	Currency             string  `json:"currency" validate:"required,len=3"`
}

// --- Response DTOs ---

// AccountResponse represents an account in API responses.
type AccountResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Balance   float64   `json:"balance"`
	Currency  string    `json:"currency"`
	Status    string    `json:"status"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BalanceResponse represents an account balance.
type BalanceResponse struct {
	Balance  float64 `json:"balance"`
	Currency string  `json:"currency"`
}

// TransactionResponse represents a transaction in API responses.
type TransactionResponse struct {
	ID              string     `json:"id"`
	AccountID       string     `json:"account_id"`
	PaymentID       *string    `json:"payment_id,omitempty"`
	TransactionType string     `json:"transaction_type"`
	Amount          float64    `json:"amount"`
	BalanceAfter    float64    `json:"balance_after"`
	Description     string     `json:"description"`
	CreatedAt       time.Time  `json:"created_at"`
}

// PaymentResponse represents a payment in API responses.
type PaymentResponse struct {
	ID                     string                 `json:"id"`
	IdempotencyKey         string                 `json:"idempotency_key"`
	PaymentType            string                 `json:"payment_type"`
	SourceAccountID        *string                `json:"source_account_id,omitempty"`
	DestinationAccountID   *string                `json:"destination_account_id,omitempty"`
	Amount                 float64                `json:"amount"`
	Currency               string                 `json:"currency"`
	Status                 string                 `json:"status"`
	Provider               *string                `json:"provider,omitempty"`
	ProviderTransactionID  *string                `json:"provider_transaction_id,omitempty"`
	RetryCount             int                    `json:"retry_count"`
	MaxRetries             int                    `json:"max_retries"`
	LastError              *string                `json:"last_error,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`
	CreatedAt              time.Time              `json:"created_at"`
	UpdatedAt              time.Time              `json:"updated_at"`
	CompletedAt            *time.Time             `json:"completed_at,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// --- Conversion helpers ---

// FromAccount converts a domain account to API response.
func FromAccount(a *account.Account) *AccountResponse {
	return &AccountResponse{
		ID:        a.ID.String(),
		UserID:    a.UserID,
		Balance:   centsToFloat(a.Balance),
		Currency:  a.Currency,
		Status:    string(a.Status),
		Version:   a.Version,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

// FromTransaction converts a domain transaction to API response.
func FromTransaction(t *account.Transaction) *TransactionResponse {
	resp := &TransactionResponse{
		ID:              t.ID.String(),
		AccountID:       t.AccountID.String(),
		TransactionType: string(t.TransactionType),
		Amount:          centsToFloat(t.Amount),
		BalanceAfter:    centsToFloat(t.BalanceAfter),
		Description:     t.Description,
		CreatedAt:       t.CreatedAt,
	}
	if t.PaymentID != nil {
		pid := t.PaymentID.String()
		resp.PaymentID = &pid
	}
	return resp
}

// FromPayment converts a domain payment to API response.
func FromPayment(p *payment.Payment) *PaymentResponse {
	resp := &PaymentResponse{
		ID:             p.ID.String(),
		IdempotencyKey: p.IdempotencyKey,
		PaymentType:    string(p.PaymentType),
		Amount:         centsToFloat(p.Amount.ValueCents),
		Currency:       p.Amount.Currency,
		Status:         string(p.Status),
		RetryCount:     p.RetryCount,
		MaxRetries:     p.MaxRetries,
		LastError:      p.LastError,
		Metadata:       p.Metadata,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
		CompletedAt:    p.CompletedAt,
	}
	if p.SourceAccountID != nil {
		sid := p.SourceAccountID.String()
		resp.SourceAccountID = &sid
	}
	if p.DestinationAccountID != nil {
		did := p.DestinationAccountID.String()
		resp.DestinationAccountID = &did
	}
	if p.Provider != nil {
		prov := string(*p.Provider)
		resp.Provider = &prov
	}
	resp.ProviderTransactionID = p.ProviderTransactionID
	return resp
}

// floatToCents converts a float dollar amount to cents.
func floatToCents(f float64) int64 {
	return int64(f * 100)
}

// centsToFloat converts cents to a float dollar amount.
func centsToFloat(cents int64) float64 {
	return float64(cents) / 100.0
}

// parseUUID parses a UUID string, returning nil if invalid.
func parseUUID(s string) *uuid.UUID {
	if s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}
