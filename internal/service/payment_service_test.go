package service

import (
	"context"
	"errors"
	"testing"

	"github.com/cassiomorais/payments/internal/domain/account"
	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/cassiomorais/payments/internal/domain/outbox"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/cassiomorais/payments/internal/providers"
	"github.com/cassiomorais/payments/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Helpers ---

func setupPaymentService() (*PaymentService, *testutil.MockPaymentRepository, *testutil.MockAccountRepository, *testutil.MockOutboxRepository, *testutil.MockTransactionManager) {
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	outboxRepo := &testutil.MockOutboxRepository{}
	txManager := testutil.NewMockTransactionManager()

	// Setup mock provider factory
	mockProvider := providers.NewMockProvider("stripe")
	providerFactory := providers.NewFactory(mockProvider)

	service := NewPaymentService(paymentRepo, accountRepo, outboxRepo, txManager, providerFactory)
	return service, paymentRepo, accountRepo, outboxRepo, txManager
}

func createTestAccount(t *testing.T, userID string, balance int64, status account.AccountStatus) *account.Account {
	t.Helper()
	acct, err := account.NewAccount(userID, balance, "USD")
	require.NoError(t, err)
	acct.Status = status
	return acct
}

// --- CreatePayment Tests ---

func TestCreatePayment_InternalTransfer_Success(t *testing.T) {
	svc, paymentRepo, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	sourceAcct := createTestAccount(t, "user1", 100000, account.StatusActive)
	destAcct := createTestAccount(t, "user2", 50000, account.StatusActive)
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	req := CreatePaymentRequest{
		IdempotencyKey:       "test-key-1",
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               10000,
		Currency:             "USD",
	}

	resp, err := svc.CreatePayment(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.IsAsync)
	assert.Equal(t, payment.StatusCompleted, resp.Payment.Status)
	assert.Equal(t, int64(10000), resp.Payment.Amount.ValueCents)

	// Verify payment was created
	stored, _ := paymentRepo.GetByID(ctx, resp.Payment.ID)
	assert.NotNil(t, stored)
	assert.Equal(t, payment.StatusCompleted, stored.Status)

	// Verify accounts were debited/credited
	sourceAfter := accountRepo.GetAccountByID(sourceAcct.ID)
	destAfter := accountRepo.GetAccountByID(destAcct.ID)
	assert.Equal(t, int64(90000), sourceAfter.Balance) // 100000 - 10000
	assert.Equal(t, int64(60000), destAfter.Balance)   // 50000 + 10000
}

func TestCreatePayment_InternalTransfer_InsufficientFunds(t *testing.T) {
	svc, _, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	sourceAcct := createTestAccount(t, "user1", 5000, account.StatusActive)
	destAcct := createTestAccount(t, "user2", 50000, account.StatusActive)
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	req := CreatePaymentRequest{
		IdempotencyKey:       "test-key-2",
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               10000, // More than source balance
		Currency:             "USD",
	}

	_, err := svc.CreatePayment(ctx, req)
	assert.ErrorIs(t, err, domainErrors.ErrInsufficientFunds)

	// Verify balances unchanged
	sourceAfter := accountRepo.GetAccountByID(sourceAcct.ID)
	destAfter := accountRepo.GetAccountByID(destAcct.ID)
	assert.Equal(t, int64(5000), sourceAfter.Balance)
	assert.Equal(t, int64(50000), destAfter.Balance)
}

func TestCreatePayment_InternalTransfer_SourceAccountNotFound(t *testing.T) {
	svc, _, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	destAcct := createTestAccount(t, "user2", 50000, account.StatusActive)
	accountRepo.AddAccount(destAcct)

	// Mock GetByID to return error for non-existent account
	nonExistentID := uuid.New()
	accountRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*account.Account, error) {
		if id == nonExistentID {
			return nil, domainErrors.ErrAccountNotFound
		}
		if id == destAcct.ID {
			return destAcct, nil
		}
		return nil, domainErrors.ErrAccountNotFound
	}

	req := CreatePaymentRequest{
		IdempotencyKey:       "test-key-3",
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &nonExistentID,
		DestinationAccountID: &destAcct.ID,
		Amount:               10000,
		Currency:             "USD",
	}

	_, err := svc.CreatePayment(ctx, req)
	assert.ErrorIs(t, err, domainErrors.ErrAccountNotFound)
}

func TestCreatePayment_InternalTransfer_DestinationAccountNotFound(t *testing.T) {
	svc, _, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	sourceAcct := createTestAccount(t, "user1", 100000, account.StatusActive)
	accountRepo.AddAccount(sourceAcct)

	// Mock GetByID to return error for non-existent account
	nonExistentID := uuid.New()
	accountRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*account.Account, error) {
		if id == sourceAcct.ID {
			return sourceAcct, nil
		}
		if id == nonExistentID {
			return nil, domainErrors.ErrAccountNotFound
		}
		return nil, domainErrors.ErrAccountNotFound
	}

	req := CreatePaymentRequest{
		IdempotencyKey:       "test-key-4",
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &nonExistentID,
		Amount:               10000,
		Currency:             "USD",
	}

	_, err := svc.CreatePayment(ctx, req)
	assert.ErrorIs(t, err, domainErrors.ErrAccountNotFound)
}

func TestCreatePayment_InternalTransfer_SourceAccountInactive(t *testing.T) {
	svc, _, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	sourceAcct := createTestAccount(t, "user1", 100000, account.StatusInactive)
	destAcct := createTestAccount(t, "user2", 50000, account.StatusActive)
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	req := CreatePaymentRequest{
		IdempotencyKey:       "test-key-5",
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               10000,
		Currency:             "USD",
	}

	_, err := svc.CreatePayment(ctx, req)
	assert.ErrorIs(t, err, domainErrors.ErrAccountInactive)
}

func TestCreatePayment_InternalTransfer_DestinationAccountInactive(t *testing.T) {
	svc, _, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	sourceAcct := createTestAccount(t, "user1", 100000, account.StatusActive)
	destAcct := createTestAccount(t, "user2", 50000, account.StatusInactive)
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	req := CreatePaymentRequest{
		IdempotencyKey:       "test-key-6",
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               10000,
		Currency:             "USD",
	}

	_, err := svc.CreatePayment(ctx, req)
	assert.ErrorIs(t, err, domainErrors.ErrAccountInactive)
}

func TestCreatePayment_InternalTransfer_CurrencyMismatch(t *testing.T) {
	svc, _, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	sourceAcct := createTestAccount(t, "user1", 100000, account.StatusActive)
	sourceAcct.Currency = "EUR" // Different currency
	accountRepo.AddAccount(sourceAcct)

	destAcct := createTestAccount(t, "user2", 50000, account.StatusActive)
	accountRepo.AddAccount(destAcct)

	req := CreatePaymentRequest{
		IdempotencyKey:       "test-key-7",
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               10000,
		Currency:             "USD", // Requesting USD but source is EUR
	}

	_, err := svc.CreatePayment(ctx, req)
	assert.ErrorIs(t, err, domainErrors.ErrInvalidCurrency)
}

func TestCreatePayment_InternalTransfer_MissingDestinationAccount(t *testing.T) {
	svc, _, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	sourceAcct := createTestAccount(t, "user1", 100000, account.StatusActive)
	accountRepo.AddAccount(sourceAcct)

	req := CreatePaymentRequest{
		IdempotencyKey:       "test-key-8",
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: nil, // Missing
		Amount:               10000,
		Currency:             "USD",
	}

	_, err := svc.CreatePayment(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required for internal transfers")
}

func TestCreatePayment_Idempotency_ReturnsExisting(t *testing.T) {
	svc, paymentRepo, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	sourceAcct := createTestAccount(t, "user1", 100000, account.StatusActive)
	destAcct := createTestAccount(t, "user2", 50000, account.StatusActive)
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	// Create first payment
	req := CreatePaymentRequest{
		IdempotencyKey:       "test-key-idempotent",
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               10000,
		Currency:             "USD",
	}

	resp1, err := svc.CreatePayment(ctx, req)
	require.NoError(t, err)
	paymentID1 := resp1.Payment.ID

	// Try to create same payment again with same idempotency key
	resp2, err := svc.CreatePayment(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, paymentID1, resp2.Payment.ID) // Same payment returned

	// Verify only one payment was created
	stored, _ := paymentRepo.GetByIdempotencyKey(ctx, "test-key-idempotent")
	assert.Equal(t, paymentID1, stored.ID)
}

func TestCreatePayment_ExternalPayment_Success(t *testing.T) {
	svc, paymentRepo, _, outboxRepo, _ := setupPaymentService()
	ctx := context.Background()

	var outboxInserted bool
	outboxRepo.InsertFunc = func(ctx context.Context, entry *outbox.Entry) error {
		outboxInserted = true
		assert.Equal(t, "payment.created", entry.EventType)
		return nil
	}

	provider := payment.ProviderStripe
	req := CreatePaymentRequest{
		IdempotencyKey:  "test-key-external-1",
		PaymentType:     payment.ExternalPayment,
		SourceAccountID: nil,
		Amount:          10000,
		Currency:        "USD",
		Provider:        &provider,
	}

	resp, err := svc.CreatePayment(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.IsAsync)
	assert.Equal(t, payment.StatusPending, resp.Payment.Status)

	// Verify payment was created
	stored, _ := paymentRepo.GetByID(ctx, resp.Payment.ID)
	assert.NotNil(t, stored)
	assert.Equal(t, payment.StatusPending, stored.Status)

	// Verify outbox entry was inserted
	assert.True(t, outboxInserted)
}

func TestCreatePayment_TransactionRollback(t *testing.T) {
	svc, paymentRepo, accountRepo, _, txManager := setupPaymentService()
	ctx := context.Background()

	sourceAcct := createTestAccount(t, "user1", 100000, account.StatusActive)
	destAcct := createTestAccount(t, "user2", 50000, account.StatusActive)
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	// Force transaction to fail
	txManager.WithTransactionFunc = func(ctx context.Context, fn func(ctx context.Context) error) error {
		return errors.New("transaction failed")
	}

	req := CreatePaymentRequest{
		IdempotencyKey:       "test-key-rollback",
		PaymentType:          payment.InternalTransfer,
		SourceAccountID:      &sourceAcct.ID,
		DestinationAccountID: &destAcct.ID,
		Amount:               10000,
		Currency:             "USD",
	}

	_, err := svc.CreatePayment(ctx, req)
	assert.Error(t, err)

	// Verify payment was not created
	stored, _ := paymentRepo.GetByIdempotencyKey(ctx, "test-key-rollback")
	assert.Nil(t, stored)

	// Verify balances unchanged
	sourceAfter := accountRepo.GetAccountByID(sourceAcct.ID)
	destAfter := accountRepo.GetAccountByID(destAcct.ID)
	assert.Equal(t, int64(100000), sourceAfter.Balance)
	assert.Equal(t, int64(50000), destAfter.Balance)
}

// --- ProcessPayment Tests ---

func TestProcessPayment_PendingToCompleted(t *testing.T) {
	svc, paymentRepo, _, _, _ := setupPaymentService()
	ctx := context.Background()

	// Create a pending external payment
	provider := payment.ProviderStripe
	p, err := payment.NewPayment("test-key", payment.ExternalPayment, nil, nil, payment.Amount{ValueCents: 10000, Currency: "USD"})
	require.NoError(t, err)
	p.SetProvider(provider)
	paymentRepo.Create(ctx, p)

	err = svc.ProcessPayment(ctx, p.ID)
	require.NoError(t, err)

	// Verify payment is completed
	stored, _ := paymentRepo.GetByID(ctx, p.ID)
	assert.Equal(t, payment.StatusCompleted, stored.Status)
	assert.NotNil(t, stored.ProviderTransactionID)
}

func TestProcessPayment_AlreadyCompleted_NoOp(t *testing.T) {
	svc, paymentRepo, _, _, _ := setupPaymentService()
	ctx := context.Background()

	// Create a completed payment
	p, err := payment.NewPayment("test-key", payment.ExternalPayment, nil, nil, payment.Amount{ValueCents: 10000, Currency: "USD"})
	require.NoError(t, err)
	p.MarkCompleted(nil)
	paymentRepo.Create(ctx, p)

	err = svc.ProcessPayment(ctx, p.ID)
	require.NoError(t, err) // Should not error

	// Verify payment still completed
	stored, _ := paymentRepo.GetByID(ctx, p.ID)
	assert.Equal(t, payment.StatusCompleted, stored.Status)
}

func TestProcessPayment_PaymentNotFound(t *testing.T) {
	svc, paymentRepo, _, _, _ := setupPaymentService()
	ctx := context.Background()

	// Mock GetByID to return error for non-existent payment
	nonExistentID := uuid.New()
	paymentRepo.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*payment.Payment, error) {
		if id == nonExistentID {
			return nil, domainErrors.ErrPaymentNotFound
		}
		return nil, domainErrors.ErrPaymentNotFound
	}

	err := svc.ProcessPayment(ctx, nonExistentID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "load payment")
}

func TestProcessPayment_ProviderFailure_MarksAsFailed(t *testing.T) {
	paymentRepo := testutil.NewMockPaymentRepository()
	accountRepo := testutil.NewMockAccountRepository()
	outboxRepo := &testutil.MockOutboxRepository{}
	txManager := testutil.NewMockTransactionManager()

	// Create a mock provider that always fails
	failingProvider := &mockFailingProvider{}
	providerFactory := providers.NewFactory(failingProvider)

	svc := NewPaymentService(paymentRepo, accountRepo, outboxRepo, txManager, providerFactory)
	ctx := context.Background()

	// Create a pending payment
	provider := payment.Provider("failing")
	p, err := payment.NewPayment("test-key", payment.ExternalPayment, nil, nil, payment.Amount{ValueCents: 10000, Currency: "USD"})
	require.NoError(t, err)
	p.SetProvider(provider)
	paymentRepo.Create(ctx, p)

	err = svc.ProcessPayment(ctx, p.ID)
	assert.Error(t, err)

	// Verify payment is marked as failed
	stored, _ := paymentRepo.GetByID(ctx, p.ID)
	assert.Equal(t, payment.StatusFailed, stored.Status)
	assert.NotNil(t, stored.LastError)
}

func TestProcessPayment_WithRetry_IncrementsRetryCount(t *testing.T) {
	svc, paymentRepo, _, _, _ := setupPaymentService()
	ctx := context.Background()

	// Create a failed payment
	provider := payment.ProviderStripe
	p, err := payment.NewPayment("test-key", payment.ExternalPayment, nil, nil, payment.Amount{ValueCents: 10000, Currency: "USD"})
	require.NoError(t, err)
	p.SetProvider(provider)
	p.MarkProcessing()
	p.MarkFailed("provider timeout")
	paymentRepo.Create(ctx, p)

	initialRetryCount := p.RetryCount
	err = svc.ProcessPayment(ctx, p.ID)
	require.NoError(t, err)

	// Verify retry count incremented
	stored, _ := paymentRepo.GetByID(ctx, p.ID)
	assert.Equal(t, initialRetryCount+1, stored.RetryCount)
	assert.Equal(t, payment.StatusCompleted, stored.Status)
}

func TestProcessPayment_WithSourceAccount_ReservesFunds(t *testing.T) {
	svc, paymentRepo, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	sourceAcct := createTestAccount(t, "user1", 100000, account.StatusActive)
	accountRepo.AddAccount(sourceAcct)

	provider := payment.ProviderStripe
	p, err := payment.NewPayment("test-key", payment.ExternalPayment, &sourceAcct.ID, nil, payment.Amount{ValueCents: 10000, Currency: "USD"})
	require.NoError(t, err)
	p.SetProvider(provider)
	paymentRepo.Create(ctx, p)

	err = svc.ProcessPayment(ctx, p.ID)
	require.NoError(t, err)

	// Verify funds were reserved (debited)
	sourceAfter := accountRepo.GetAccountByID(sourceAcct.ID)
	assert.Equal(t, int64(90000), sourceAfter.Balance)

	// Verify payment completed
	stored, _ := paymentRepo.GetByID(ctx, p.ID)
	assert.Equal(t, payment.StatusCompleted, stored.Status)
}

// --- RefundPayment Tests ---

func TestRefundPayment_Success(t *testing.T) {
	svc, paymentRepo, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	sourceAcct := createTestAccount(t, "user1", 50000, account.StatusActive)
	accountRepo.AddAccount(sourceAcct)

	// Create a completed external payment
	provider := payment.ProviderStripe
	p, err := payment.NewPayment("test-key", payment.ExternalPayment, &sourceAcct.ID, nil, payment.Amount{ValueCents: 10000, Currency: "USD"})
	require.NoError(t, err)
	p.SetProvider(provider)
	p.MarkCompleted(nil)
	paymentRepo.Create(ctx, p)

	refunded, err := svc.RefundPayment(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, payment.StatusRefunded, refunded.Status)

	// Verify funds were credited back
	sourceAfter := accountRepo.GetAccountByID(sourceAcct.ID)
	assert.Equal(t, int64(60000), sourceAfter.Balance) // 50000 + 10000
}

func TestRefundPayment_NonCompletedPayment_Error(t *testing.T) {
	svc, paymentRepo, _, _, _ := setupPaymentService()
	ctx := context.Background()

	// Create a pending payment
	p, err := payment.NewPayment("test-key", payment.ExternalPayment, nil, nil, payment.Amount{ValueCents: 10000, Currency: "USD"})
	require.NoError(t, err)
	paymentRepo.Create(ctx, p)

	_, err = svc.RefundPayment(ctx, p.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot refund payment")
}

func TestRefundPayment_InternalTransfer_ReversesAccounts(t *testing.T) {
	svc, paymentRepo, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	sourceAcct := createTestAccount(t, "user1", 90000, account.StatusActive)
	destAcct := createTestAccount(t, "user2", 60000, account.StatusActive)
	accountRepo.AddAccount(sourceAcct)
	accountRepo.AddAccount(destAcct)

	// Create a completed internal transfer
	p, err := payment.NewPayment("test-key", payment.InternalTransfer, &sourceAcct.ID, &destAcct.ID, payment.Amount{ValueCents: 10000, Currency: "USD"})
	require.NoError(t, err)
	p.MarkCompleted(nil)
	paymentRepo.Create(ctx, p)

	refunded, err := svc.RefundPayment(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, payment.StatusRefunded, refunded.Status)

	// Verify accounts reversed
	sourceAfter := accountRepo.GetAccountByID(sourceAcct.ID)
	destAfter := accountRepo.GetAccountByID(destAcct.ID)
	assert.Equal(t, int64(100000), sourceAfter.Balance) // 90000 + 10000 (refunded)
	assert.Equal(t, int64(50000), destAfter.Balance)    // 60000 - 10000 (reversed)
}

// --- Helper Account Operations Tests ---

func TestDebitAccount_Success(t *testing.T) {
	svc, _, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	acct := createTestAccount(t, "user1", 100000, account.StatusActive)
	accountRepo.AddAccount(acct)

	paymentID := uuid.New()
	balanceAfter, err := svc.debitAccount(ctx, acct.ID, paymentID, 10000, "test debit")
	require.NoError(t, err)
	assert.Equal(t, int64(90000), balanceAfter)

	// Verify account updated
	updated := accountRepo.GetAccountByID(acct.ID)
	assert.Equal(t, int64(90000), updated.Balance)
	assert.Equal(t, 1, updated.Version)
}

func TestCreditAccount_Success(t *testing.T) {
	svc, _, accountRepo, _, _ := setupPaymentService()
	ctx := context.Background()

	acct := createTestAccount(t, "user1", 100000, account.StatusActive)
	accountRepo.AddAccount(acct)

	paymentID := uuid.New()
	balanceAfter, err := svc.creditAccount(ctx, acct.ID, paymentID, 10000, "test credit")
	require.NoError(t, err)
	assert.Equal(t, int64(110000), balanceAfter)

	// Verify account updated
	updated := accountRepo.GetAccountByID(acct.ID)
	assert.Equal(t, int64(110000), updated.Balance)
	assert.Equal(t, 1, updated.Version)
}

// --- Mock Failing Provider ---

type mockFailingProvider struct{}

func (m *mockFailingProvider) Name() string {
	return "failing"
}

func (m *mockFailingProvider) ProcessPayment(ctx context.Context, req providers.ProcessRequest) (*providers.ProviderResult, error) {
	return nil, errors.New("provider is down")
}

func (m *mockFailingProvider) RefundPayment(ctx context.Context, req providers.RefundRequest) (*providers.ProviderResult, error) {
	return nil, errors.New("refund failed")
}

