package providers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMockProvider(t *testing.T) {
	provider := NewMockProvider("test")

	assert.NotNil(t, provider)
	assert.Equal(t, "test", provider.Name())
}

func TestMockProvider_WithLatency(t *testing.T) {
	latency := 100 * time.Millisecond
	provider := NewMockProvider("test", WithLatency(latency))

	assert.NotNil(t, provider)
	// Latency is internal, just verify provider was created
}

func TestMockProvider_WithFailureRate(t *testing.T) {
	provider := NewMockProvider("test", WithFailureRate(0.5))

	assert.NotNil(t, provider)
	// Failure rate is internal, just verify provider was created
}

func TestMockProvider_ProcessPayment_Success(t *testing.T) {
	// Create a provider with 0% failure rate
	provider := NewMockProvider("test", WithFailureRate(0.0))
	ctx := context.Background()

	req := ProcessRequest{
		PaymentID:   "pay_123",
		AmountCents: 10000,
		Currency:    "USD",
		Metadata:    map[string]any{"customer": "cust_123"},
	}

	result, err := provider.ProcessPayment(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "success", result.Status)
	assert.NotEmpty(t, result.TransactionID)
	assert.Contains(t, result.TransactionID, "test_")
}

func TestMockProvider_ProcessPayment_Failure(t *testing.T) {
	// Create a provider with 100% failure rate
	provider := NewMockProvider("test", WithFailureRate(1.0))
	ctx := context.Background()

	req := ProcessRequest{
		PaymentID:   "pay_123",
		AmountCents: 10000,
		Currency:    "USD",
	}

	result, err := provider.ProcessPayment(ctx, req)
	assert.Error(t, err) // Provider returns error on failure
	assert.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.ErrorMessage, "simulated")
}

func TestMockProvider_RefundPayment_Success(t *testing.T) {
	provider := NewMockProvider("test", WithFailureRate(0.0))
	ctx := context.Background()

	req := RefundRequest{
		PaymentID:     "pay_123",
		TransactionID: "txn_456",
		AmountCents:   10000,
		Currency:      "USD",
	}

	result, err := provider.RefundPayment(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "success", result.Status)
	assert.NotEmpty(t, result.TransactionID)
	assert.Contains(t, result.TransactionID, "test_refund_")
}

func TestMockProvider_RefundPayment_Failure(t *testing.T) {
	provider := NewMockProvider("test", WithFailureRate(1.0))
	ctx := context.Background()

	req := RefundRequest{
		PaymentID:     "pay_123",
		TransactionID: "txn_456",
		AmountCents:   10000,
		Currency:      "USD",
	}

	result, err := provider.RefundPayment(ctx, req)
	assert.Error(t, err) // Provider returns error on failure
	assert.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.ErrorMessage, "simulated")
}

func TestMockProvider_Latency(t *testing.T) {
	latency := 50 * time.Millisecond
	provider := NewMockProvider("test", WithLatency(latency), WithFailureRate(0.0))
	ctx := context.Background()

	req := ProcessRequest{
		PaymentID:   "pay_123",
		AmountCents: 10000,
		Currency:    "USD",
	}

	start := time.Now()
	_, err := provider.ProcessPayment(ctx, req)
	duration := time.Since(start)

	require.NoError(t, err)
	// Should take at least the specified latency
	assert.GreaterOrEqual(t, duration, latency)
}
