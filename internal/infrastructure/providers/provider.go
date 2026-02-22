package providers

import (
	"context"
)

// ProviderResult holds the result of an external provider call.
type ProviderResult struct {
	TransactionID string
	Status        string // "success", "failed", "pending"
	ErrorMessage  string
}

// Provider is the interface that external payment providers implement.
type Provider interface {
	// Name returns the provider name.
	Name() string
	// ProcessPayment processes a payment through the provider.
	ProcessPayment(ctx context.Context, req ProcessRequest) (*ProviderResult, error)
	// RefundPayment refunds a payment through the provider.
	RefundPayment(ctx context.Context, req RefundRequest) (*ProviderResult, error)
}

// ProcessRequest contains the data needed to process a payment.
type ProcessRequest struct {
	PaymentID   string
	AmountCents int64 // in cents
	Currency    string
	Metadata    map[string]interface{}
}

// RefundRequest contains the data needed to refund a payment.
type RefundRequest struct {
	PaymentID     string
	TransactionID string
	AmountCents   int64 // in cents
	Currency      string
}
