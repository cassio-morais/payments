package payment_test

import (
	"testing"

	"github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validSourceID() *uuid.UUID {
	id := uuid.New()
	return &id
}

func validDestID() *uuid.UUID {
	id := uuid.New()
	return &id
}

func TestNewPayment_Valid(t *testing.T) {
	p, err := payment.NewPayment("key-1", payment.InternalTransfer, validSourceID(), validDestID(), payment.Amount{ValueCents: 10000, Currency: "USD"})
	require.NoError(t, err)
	assert.Equal(t, payment.StatusPending, p.Status)
	assert.Equal(t, "key-1", p.IdempotencyKey)
	assert.Equal(t, int64(10000), p.Amount.ValueCents)
	assert.Equal(t, "USD", p.Amount.Currency)
	assert.Equal(t, 0, p.RetryCount)
	assert.Equal(t, 3, p.MaxRetries)
}

func TestNewPayment_InvalidAmount(t *testing.T) {
	_, err := payment.NewPayment("key-1", payment.InternalTransfer, validSourceID(), validDestID(), payment.Amount{ValueCents: -1000, Currency: "USD"})
	assert.Error(t, err)
}

func TestNewPayment_ZeroAmount(t *testing.T) {
	_, err := payment.NewPayment("key-1", payment.InternalTransfer, validSourceID(), validDestID(), payment.Amount{ValueCents: 0, Currency: "USD"})
	assert.Error(t, err)
}

func TestNewPayment_EmptyCurrency(t *testing.T) {
	_, err := payment.NewPayment("key-1", payment.InternalTransfer, validSourceID(), validDestID(), payment.Amount{ValueCents: 1000, Currency: ""})
	assert.Error(t, err)
}

func TestNewPayment_InvalidCurrencyLength(t *testing.T) {
	_, err := payment.NewPayment("key-1", payment.InternalTransfer, validSourceID(), validDestID(), payment.Amount{ValueCents: 1000, Currency: "US"})
	assert.Error(t, err)
}

func TestNewPayment_EmptyIdempotencyKey(t *testing.T) {
	_, err := payment.NewPayment("", payment.InternalTransfer, validSourceID(), validDestID(), payment.Amount{ValueCents: 1000, Currency: "USD"})
	assert.ErrorIs(t, err, errors.ErrInvalidInput)
}

func TestAmount_String(t *testing.T) {
	a := payment.Amount{ValueCents: 10050, Currency: "USD"}
	assert.Equal(t, "100.50 USD", a.String())

	a2 := payment.Amount{ValueCents: 5000, Currency: "EUR"}
	assert.Equal(t, "50.00 EUR", a2.String())
}

func TestAmount_Validate(t *testing.T) {
	valid := payment.Amount{ValueCents: 100, Currency: "USD"}
	assert.NoError(t, valid.Validate())

	invalid := payment.Amount{ValueCents: 0, Currency: "USD"}
	assert.Error(t, invalid.Validate())
}

// --- State Machine Tests ---

func newPendingPayment(t *testing.T) *payment.Payment {
	t.Helper()
	p, err := payment.NewPayment("key-"+uuid.New().String(), payment.ExternalPayment, validSourceID(), nil, payment.Amount{ValueCents: 5000, Currency: "USD"})
	require.NoError(t, err)
	return p
}

func TestStateMachine_PendingToProcessing(t *testing.T) {
	p := newPendingPayment(t)
	assert.NoError(t, p.MarkProcessing())
	assert.Equal(t, payment.StatusProcessing, p.Status)
}

func TestStateMachine_PendingToCompleted(t *testing.T) {
	p := newPendingPayment(t)
	assert.NoError(t, p.MarkCompleted(nil))
	assert.Equal(t, payment.StatusCompleted, p.Status)
	assert.NotNil(t, p.CompletedAt)
}

func TestStateMachine_PendingToCancelled(t *testing.T) {
	p := newPendingPayment(t)
	assert.NoError(t, p.MarkCancelled())
	assert.Equal(t, payment.StatusCancelled, p.Status)
}

func TestStateMachine_ProcessingToCompleted(t *testing.T) {
	p := newPendingPayment(t)
	require.NoError(t, p.MarkProcessing())
	txID := "txn_123"
	assert.NoError(t, p.MarkCompleted(&txID))
	assert.Equal(t, payment.StatusCompleted, p.Status)
	assert.Equal(t, &txID, p.ProviderTransactionID)
}

func TestStateMachine_ProcessingToFailed(t *testing.T) {
	p := newPendingPayment(t)
	require.NoError(t, p.MarkProcessing())
	assert.NoError(t, p.MarkFailed("provider timeout"))
	assert.Equal(t, payment.StatusFailed, p.Status)
	assert.Equal(t, "provider timeout", *p.LastError)
}

func TestStateMachine_FailedToProcessing_Retry(t *testing.T) {
	p := newPendingPayment(t)
	require.NoError(t, p.MarkProcessing())
	require.NoError(t, p.MarkFailed("error"))

	assert.True(t, p.CanRetry())
	assert.NoError(t, p.IncrementRetry())
	assert.NoError(t, p.MarkProcessing())
	assert.Equal(t, payment.StatusProcessing, p.Status)
	assert.Equal(t, 1, p.RetryCount)
}

func TestStateMachine_CompletedToRefunded(t *testing.T) {
	p := newPendingPayment(t)
	require.NoError(t, p.MarkCompleted(nil))
	assert.NoError(t, p.MarkRefunded())
	assert.Equal(t, payment.StatusRefunded, p.Status)
}

func TestStateMachine_InvalidTransition_CancelledToProcessing(t *testing.T) {
	p := newPendingPayment(t)
	require.NoError(t, p.MarkCancelled())
	assert.Error(t, p.MarkProcessing())
}

func TestStateMachine_InvalidTransition_CompletedToProcessing(t *testing.T) {
	p := newPendingPayment(t)
	require.NoError(t, p.MarkCompleted(nil))
	assert.Error(t, p.MarkProcessing())
}

func TestStateMachine_InvalidTransition_RefundedToAnything(t *testing.T) {
	p := newPendingPayment(t)
	require.NoError(t, p.MarkCompleted(nil))
	require.NoError(t, p.MarkRefunded())
	assert.Error(t, p.MarkCompleted(nil))
	assert.Error(t, p.MarkProcessing())
	assert.Error(t, p.MarkFailed("x"))
}

// --- Terminal state ---

func TestIsTerminal(t *testing.T) {
	p := newPendingPayment(t)
	assert.False(t, p.IsTerminal())

	require.NoError(t, p.MarkCompleted(nil))
	assert.True(t, p.IsTerminal())
}

// --- Retry logic ---

func TestMaxRetries(t *testing.T) {
	p := newPendingPayment(t)
	p.MaxRetries = 2

	require.NoError(t, p.MarkProcessing())
	require.NoError(t, p.MarkFailed("err"))

	assert.NoError(t, p.IncrementRetry()) // 1
	assert.NoError(t, p.IncrementRetry()) // 2
	assert.ErrorIs(t, p.IncrementRetry(), errors.ErrMaxRetriesExceeded)
	assert.False(t, p.CanRetry())
}
