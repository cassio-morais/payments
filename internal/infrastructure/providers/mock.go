package providers

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/google/uuid"
)

// MockProvider is a configurable mock payment provider for testing.
type MockProvider struct {
	name        string
	failureRate float64 // 0.0 to 1.0
	latency     time.Duration
	timeoutRate float64 // 0.0 to 1.0
}

// MockProviderOption configures a MockProvider.
type MockProviderOption func(*MockProvider)

// WithFailureRate sets the probability that a call will return an error.
func WithFailureRate(rate float64) MockProviderOption {
	return func(p *MockProvider) { p.failureRate = rate }
}

// WithLatency sets the simulated processing latency.
func WithLatency(d time.Duration) MockProviderOption {
	return func(p *MockProvider) { p.latency = d }
}

// WithTimeoutRate sets the probability of a simulated timeout.
func WithTimeoutRate(rate float64) MockProviderOption {
	return func(p *MockProvider) { p.timeoutRate = rate }
}

// NewMockProvider creates a new mock provider.
func NewMockProvider(name string, opts ...MockProviderOption) *MockProvider {
	p := &MockProvider{
		name:        name,
		failureRate: 0.0,
		latency:     100 * time.Millisecond,
		timeoutRate: 0.0,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func (p *MockProvider) Name() string { return p.name }

func (p *MockProvider) ProcessPayment(ctx context.Context, req ProcessRequest) (*ProviderResult, error) {
	// Simulate latency
	select {
	case <-time.After(p.latency):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Simulate timeout
	if rand.Float64() < p.timeoutRate {
		return nil, domainErrors.ErrProviderTimeout
	}

	// Simulate failure
	if rand.Float64() < p.failureRate {
		return &ProviderResult{
			Status:       "failed",
			ErrorMessage: fmt.Sprintf("%s: simulated processing failure for payment %s", p.name, req.PaymentID),
		}, domainErrors.ErrProviderRejected
	}

	return &ProviderResult{
		TransactionID: fmt.Sprintf("%s_txn_%s", p.name, uuid.New().String()[:8]),
		Status:        "success",
	}, nil
}

func (p *MockProvider) RefundPayment(ctx context.Context, req RefundRequest) (*ProviderResult, error) {
	select {
	case <-time.After(p.latency):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if rand.Float64() < p.failureRate {
		return &ProviderResult{
			Status:       "failed",
			ErrorMessage: fmt.Sprintf("%s: simulated refund failure", p.name),
		}, domainErrors.ErrProviderRejected
	}

	return &ProviderResult{
		TransactionID: fmt.Sprintf("%s_refund_%s", p.name, uuid.New().String()[:8]),
		Status:        "success",
	}, nil
}
