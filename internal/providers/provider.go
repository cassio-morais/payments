package providers

import (
	"context"
)

type ProviderResult struct {
	TransactionID string
	Status        string // "success", "failed", "pending"
	ErrorMessage  string
}

type Provider interface {
	// Name returns the provider name.
	Name() string
	// ProcessPayment processes a payment through the provider.
	ProcessPayment(ctx context.Context, req ProcessRequest) (*ProviderResult, error)
	// RefundPayment refunds a payment through the provider.
	RefundPayment(ctx context.Context, req RefundRequest) (*ProviderResult, error)
}

type ProcessRequest struct {
	PaymentID   string
	AmountCents int64 // in cents
	Currency    string
	Metadata    map[string]any
}

type RefundRequest struct {
	PaymentID     string
	TransactionID string
	AmountCents   int64 // in cents
	Currency      string
}
